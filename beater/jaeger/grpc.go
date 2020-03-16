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
	"fmt"
	"strconv"

	"go.uber.org/zap"

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
)

var (
	gRPCCollectorRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.collect", monitoring.PublishExpvar)
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
func (c grpcCollector) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	gRPCCollectorMonitoringMap.inc(request.IDRequestCount)
	defer gRPCCollectorMonitoringMap.inc(request.IDResponseCount)

	if err := c.postSpans(ctx, r.Batch); err != nil {
		gRPCCollectorMonitoringMap.inc(request.IDResponseErrorsCount)
		c.log.With(zap.Error(err)).Error("error gRPC PostSpans")
		return nil, err
	}
	gRPCCollectorMonitoringMap.inc(request.IDResponseValidCount)
	return &api_v2.PostSpansResponse{}, nil
}

func (c grpcCollector) postSpans(ctx context.Context, batch model.Batch) error {
	if err := c.auth(ctx, batch); err != nil {
		gRPCCollectorMonitoringMap.inc(request.IDResponseErrorsUnauthorized)
		return status.Error(codes.Unauthenticated, err.Error())
	}
	return consumeBatch(ctx, batch, c.consumer, gRPCCollectorMonitoringMap)
}

const samplingRateKey = "transaction_sample_rate"

var (
	gRPCSamplingRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.sampling", monitoring.PublishExpvar)
	gRPCSamplingMonitoringMap monitoringMap = request.MonitoringMapForRegistry(gRPCSamplingRegistry, monitoringKeys)
)

type grpcSampler struct {
	log         *logp.Logger
	defaultRate float64
	client      kibana.Client
	fetcher     *agentcfg.Fetcher
}

// GetSamplingStrategy implements the api_v2/sampling.proto.
// Only probabilistic sampling is supported.
// It fetches the sampling rate from the central configuration management and returns the sampling strategy.
func (s grpcSampler) GetSamplingStrategy(
	ctx context.Context,
	params *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	gRPCSamplingMonitoringMap.inc(request.IDRequestCount)
	defer gRPCSamplingMonitoringMap.inc(request.IDResponseCount)
	samplingRate, err := s.fetchSamplingRate(ctx, params.ServiceName)
	if err != nil {
		s.log.With(zap.Error(err)).Error("Fetching sampling rate failed, falling back to default value.")
		samplingRate = s.defaultRate
	}
	gRPCSamplingMonitoringMap.inc(request.IDResponseValidCount)
	return &api_v2.SamplingStrategyResponse{
		StrategyType:          api_v2.SamplingStrategyType_PROBABILISTIC,
		ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{SamplingRate: samplingRate}}, nil
}

func (s grpcSampler) fetchSamplingRate(ctx context.Context, service string) (float64, error) {
	if err := s.validateKibanaClient(ctx); err != nil {
		return 0, err
	}
	result, err := s.fetcher.Fetch(ctx, agentcfg.NewQuery(service, ""))
	if err != nil {
		return 0, fmt.Errorf("fetching sampling rate from Kibana client failed: %w", err)
	}

	if sr, ok := result.Source.Settings[samplingRateKey]; ok {
		srFloat64, err := strconv.ParseFloat(sr, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing error for sample rate `%v`: %w", sr, err)
		}
		return srFloat64, nil
	}
	s.log.Debugf("No sampling rate found for %v, falling back to default value.", service)
	return s.defaultRate, nil
}

func (s grpcSampler) validateKibanaClient(ctx context.Context) error {
	supported, err := s.client.SupportsVersion(ctx, kibana.AgentConfigMinVersion, true)
	if err != nil {
		return fmt.Errorf("error checking kibana version: %w", err)
	}
	if !supported {
		version := "unknown"
		if ver, err := s.client.GetVersion(ctx); err == nil {
			version = ver.String()
		}
		return fmt.Errorf("not supported by Kibana version %v, min Kibana version: %v, ",
			version,
			kibana.AgentConfigMinVersion)
	}
	return nil
}
