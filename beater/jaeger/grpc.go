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
	"strings"

	jaegermodel "github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"go.opentelemetry.io/collector/consumer"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
)

var (
	gRPCCollectorRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.collect")
	gRPCCollectorMonitoringMap monitoringMap = request.MonitoringMapForRegistry(
		gRPCCollectorRegistry, append(request.DefaultResultIDs,
			request.IDResponseErrorsRateLimit,
			request.IDResponseErrorsTimeout,
			request.IDResponseErrorsUnauthorized,
		),
	)

	// RegistryMonitoringMaps provides mappings from the fully qualified gRPC
	// method name to its respective monitoring map.
	RegistryMonitoringMaps = map[string]map[request.ResultID]*monitoring.Int{
		postSpansFullMethod:           gRPCCollectorMonitoringMap,
		getSamplingStrategyFullMethod: gRPCSamplingMonitoringMap,
	}
)

const (
	postSpansFullMethod           = "/jaeger.api_v2.CollectorService/PostSpans"
	getSamplingStrategyFullMethod = "/jaeger.api_v2.SamplingManager/GetSamplingStrategy"

	// elasticAuthTag is the name of the agent tag that will be used for auth.
	// The tag value should be "Bearer <secret token" or "ApiKey <api key>".
	elasticAuthTag = "elastic-apm-auth"
)

// RegisterGRPCServices registers Jaeger gRPC services with srv.
func RegisterGRPCServices(
	srv *grpc.Server,
	logger *logp.Logger,
	processor model.BatchProcessor,
	fetcher agentcfg.Fetcher,
) {
	traceConsumer := &otel.Consumer{Processor: processor}
	api_v2.RegisterCollectorServiceServer(srv, &grpcCollector{traceConsumer})
	api_v2.RegisterSamplingManagerServer(srv, &grpcSampler{logger, fetcher})
}

// MethodAuthenticators returns a map of all supported Jaeger/gRPC methods to authorization handlers.
func MethodAuthenticators(authenticator *auth.Authenticator) map[string]interceptors.MethodAuthenticator {
	return map[string]interceptors.MethodAuthenticator{
		postSpansFullMethod:           postSpansMethodAuthenticator(authenticator),
		getSamplingStrategyFullMethod: getSamplingStrategyMethodAuthenticator(authenticator),
	}
}

// grpcCollector implements Jaeger api_v2 protocol for receiving tracing data
type grpcCollector struct {
	consumer consumer.Traces
}

// PostSpans implements the api_v2/collector.proto. It converts spans received via Jaeger Proto batch to open-telemetry
// TraceData and passes them on to the internal Consumer taking care of converting into Elastic APM format.
// The implementation of the protobuf contract is based on the open-telemetry implementation at
// https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/jaegerreceiver
func (c *grpcCollector) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	if err := c.postSpans(ctx, r.Batch); err != nil {
		return nil, err
	}
	return &api_v2.PostSpansResponse{}, nil
}

func (c *grpcCollector) postSpans(ctx context.Context, batch jaegermodel.Batch) error {
	spanCount := int64(len(batch.Spans))
	gRPCCollectorMonitoringMap.add(request.IDEventReceivedCount, spanCount)
	traces := jaegertranslator.ProtoBatchToInternalTraces(batch)
	return c.consumer.ConsumeTraces(ctx, traces)
}

var (
	gRPCSamplingRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc.sampling")
	gRPCSamplingMonitoringMap monitoringMap = request.MonitoringMapForRegistry(
		gRPCSamplingRegistry, append(request.DefaultResultIDs, request.IDEventReceivedCount),
	)

	jaegerAgentPrefixes = []string{otel.AgentNameJaeger}
)

type grpcSampler struct {
	logger  *logp.Logger
	fetcher agentcfg.Fetcher
}

// GetSamplingStrategy implements the api_v2/sampling.proto.
// Only probabilistic sampling is supported.
// It fetches the sampling rate from the central configuration management and returns the sampling strategy.
func (s *grpcSampler) GetSamplingStrategy(
	ctx context.Context,
	params *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {

	samplingRate, err := s.fetchSamplingRate(ctx, params.ServiceName)
	if err != nil {
		var verr *agentcfg.ValidationError
		if errors.As(err, &verr) {
			if err := checkValidationError(verr); err != nil {
				// do not return full error details since this is part of an unprotected endpoint response
				s.logger.With(logp.Error(err)).Error("Configured Kibana client does not support agent remote configuration")
				return nil, errors.New("agent remote configuration not supported, check server logs for more details")
			}
		}

		// do not return full error details since this is part of an unprotected endpoint response
		s.logger.With(logp.Error(err)).Error("No valid sampling rate fetched from Kibana.")
		return nil, errors.New("no sampling rate available, check server logs for more details")
	}

	return &api_v2.SamplingStrategyResponse{
		StrategyType:          api_v2.SamplingStrategyType_PROBABILISTIC,
		ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{SamplingRate: samplingRate},
	}, nil
}

func (s *grpcSampler) fetchSamplingRate(ctx context.Context, service string) (float64, error) {
	// Only service, and not agent, is known for config queries.
	// For anonymous/untrusted agents, we filter the results using
	// query.InsecureAgents below.
	authResource := auth.Resource{ServiceName: service}
	if err := auth.Authorize(ctx, auth.ActionAgentConfig, authResource); err != nil {
		return 0, err
	}

	query := agentcfg.Query{
		Service:              agentcfg.Service{Name: service},
		InsecureAgents:       jaegerAgentPrefixes,
		MarkAsAppliedByAgent: true,
	}
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

func checkValidationError(err *agentcfg.ValidationError) error {
	body := err.Body()
	switch {
	case strings.HasPrefix(body, agentcfg.ErrMsgKibanaDisabled):
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return errors.New("jaeger remote sampling endpoint is disabled, " +
			"configure the `apm-server.kibana` section in apm-server.yml to enable it")
	case strings.HasPrefix(body, agentcfg.ErrMsgNoKibanaConnection):
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return fmt.Errorf("error checking kibana version: %w", err)
	case strings.HasPrefix(body, agentcfg.ErrMsgKibanaVersionNotCompatible):
		gRPCSamplingMonitoringMap.inc(request.IDResponseErrorsServiceUnavailable)
		return fmt.Errorf(
			"not supported by used Kibana version, min required Kibana version: %v",
			agentcfg.KibanaMinVersion,
		)
	default:
		return nil
	}
}

func postSpansMethodAuthenticator(authenticator *auth.Authenticator) interceptors.MethodAuthenticator {
	return func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error) {
		postSpansRequest := req.(*api_v2.PostSpansRequest)
		batch := &postSpansRequest.Batch
		var kind, token string
		for i, kv := range batch.Process.GetTags() {
			if kv.Key != elasticAuthTag {
				continue
			}
			// Remove the auth tag.
			batch.Process.Tags = append(batch.Process.Tags[:i], batch.Process.Tags[i+1:]...)
			kind, token = auth.ParseAuthorizationHeader(kv.VStr)
			break
		}
		return authenticator.Authenticate(ctx, kind, token)
	}
}

func getSamplingStrategyMethodAuthenticator(authenticator *auth.Authenticator) interceptors.MethodAuthenticator {
	// Sampling strategy queries are always unauthenticated. We still consult
	// the authenticator in case auth isn't required, in which case we should
	// not rate limit the request.
	anonymousAuthenticator, err := auth.NewAuthenticator(config.AgentAuth{
		Anonymous: config.AnonymousAgentAuth{Enabled: true},
	})
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error) {
		details, authz, err := authenticator.Authenticate(ctx, "", "")
		if !errors.Is(err, auth.ErrAuthFailed) {
			return details, authz, err
		}
		return anonymousAuthenticator.Authenticate(ctx, "", "")
	}
}

type monitoringMap map[request.ResultID]*monitoring.Int

func (m monitoringMap) inc(id request.ResultID) {
	if counter, ok := m[id]; ok {
		counter.Inc()
	}
}

func (m monitoringMap) add(id request.ResultID, n int64) {
	if counter, ok := m[id]; ok {
		counter.Add(n)
	}
}
