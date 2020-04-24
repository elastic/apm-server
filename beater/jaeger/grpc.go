// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package jaeger

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/processor/otel"
)

var (
	gRPCCollectorRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.collect")
	gRPCCollectorMonitoringMap monitoringMap = request.MonitoringMapForRegistry(gRPCCollectorRegistry, monitoringKeys)
)

// grpcCollector implements Jaeger api_v2 protocol for receiving tracing data
type grpcCollector struct {
	log      *logp.Logger
	auth     authFunc
	consumer consumer.TraceConsumer
}

// PostSpans implements the api_v2/collector.proto. It converts spans received via Jaeger Proto batch to open-telemetry
// TraceData and passes them on to the internal Consumer taking care of converting into Elastic APM format.
// The implementation of the protobuf contract is based on the open-telemetry implementation at
// https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/jaegerreceiver
func (c *grpcCollector) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	gRPCCollectorMonitoringMap.inc(request.IDRequestCount)
	defer gRPCCollectorMonitoringMap.inc(request.IDResponseCount)

	if err := c.postSpans(ctx, r.Batch); err != nil {
		gRPCCollectorMonitoringMap.inc(request.IDResponseErrorsCount)
		c.log.With(logp.Error(err)).Error("error gRPC PostSpans")
		return nil, err
	}
	gRPCCollectorMonitoringMap.inc(request.IDResponseValidCount)
	return &api_v2.PostSpansResponse{}, nil
}

func (c *grpcCollector) postSpans(ctx context.Context, batch model.Batch) error {
	if err := c.auth(ctx, batch); err != nil {
		gRPCCollectorMonitoringMap.inc(request.IDResponseErrorsUnauthorized)
		return status.Error(codes.Unauthenticated, err.Error())
	}
	return consumeBatch(ctx, batch, c.consumer, gRPCCollectorMonitoringMap)
}

var (
	gRPCSamplingRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.sampling")
	gRPCSamplingMonitoringMap monitoringMap = request.MonitoringMapForRegistry(gRPCSamplingRegistry, monitoringKeys)

	jaegerAgentPrefixes = []string{otel.AgentNameJaeger}
)

type grpcSampler struct {
	log     *logp.Logger
	client  kibana.Client
	fetcher *agentcfg.Fetcher
}

// GetSamplingStrategy implements the api_v2/sampling.proto.
// Only probabilistic sampling is supported.
// It fetches the sampling rate from the central configuration management and returns the sampling strategy.
func (s *grpcSampler) GetSamplingStrategy(
	ctx context.Context,
	params *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {

	gRPCSamplingMonitoringMap.inc(request.IDRequestCount)
	defer gRPCSamplingMonitoringMap.inc(request.IDResponseCount)
	if err := s.validateKibanaClient(ctx); err != nil {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsCount)
		// do not return full error details since this is part of an unprotected endpoint response
		s.log.With(logp.Error(err)).Error("Configured Kibana client does not support agent remote configuration")
		return nil, errors.New("agent remote configuration not supported, check server logs for more details")
	}
	samplingRate, err := s.fetchSamplingRate(ctx, params.ServiceName)
	if err != nil {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsCount)
		// do not return full error details since this is part of an unprotected endpoint response
		s.log.With(logp.Error(err)).Error("No valid sampling rate fetched from Kibana.")
		return nil, errors.New("no sampling rate available, check server logs for more details")
	}
	gRPCSamplingMonitoringMap.inc(request.IDResponseValidCount)
	return &api_v2.SamplingStrategyResponse{
		StrategyType:          api_v2.SamplingStrategyType_PROBABILISTIC,
		ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{SamplingRate: samplingRate}}, nil
}

func (s *grpcSampler) fetchSamplingRate(ctx context.Context, service string) (float64, error) {
	query := agentcfg.Query{Service: agentcfg.Service{Name: service},
		InsecureAgents: jaegerAgentPrefixes, MarkAsAppliedByAgent: newBool(true)}
	result, err := s.fetcher.Fetch(ctx, query)
	if err != nil {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return 0, fmt.Errorf("fetching sampling rate failed: %w", err)
	}

	if sr, ok := result.Source.Settings[agentcfg.TransactionSamplingRateKey]; ok {
		srFloat64, err := strconv.ParseFloat(sr, 64)
		if err != nil {
			gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsInternal)
			return 0, fmt.Errorf("parsing error for sampling rate `%v`: %w", sr, err)
		}
		return srFloat64, nil
	}
	gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsNotFound)
	return 0, fmt.Errorf("no sampling rate found for %v", service)
}

func (s *grpcSampler) validateKibanaClient(ctx context.Context) error {
	if s.client == nil {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return errors.New("jaeger remote sampling endpoint is disabled, " +
			"configure the `apm-server.kibana` section in apm-server.yml to enable it")
	}
	supported, err := s.client.SupportsVersion(ctx, agentcfg.KibanaMinVersion, true)
	if err != nil {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return fmt.Errorf("error checking kibana version: %w", err)
	}
	if !supported {
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return fmt.Errorf("not supported by used Kibana version, min required Kibana version: %v",
			agentcfg.KibanaMinVersion)
	}
	return nil
}

func newBool(b bool) *bool { return &b }
