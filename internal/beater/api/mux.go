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

package api

import (
	"net/http"
	httppprof "net/http/pprof"
	"regexp"
	"runtime/pprof"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/input/elasticapm"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/api/config/agent"
	"github.com/elastic/apm-server/internal/beater/api/intake"
	"github.com/elastic/apm-server/internal/beater/api/root"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/middleware"
	"github.com/elastic/apm-server/internal/beater/otlp"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/logs"
	srvmodelprocessor "github.com/elastic/apm-server/internal/model/modelprocessor"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/apm-server/internal/version"
)

const (
	// RootPath defines the server's root path
	RootPath = "/"

	// Backend routes

	// AgentConfigPath defines the path to query for agent config management
	AgentConfigPath = "/config/v1/agents"
	// IntakePath defines the path to ingest monitored events
	IntakePath = "/intake/v2/events"

	// RUM routes

	// AgentConfigRUMPath defines the path to query for the RUM agent config management
	AgentConfigRUMPath = "/config/v1/rum/agents"
	// IntakeRUMPath defines the path to ingest monitored RUM events
	IntakeRUMPath = "/intake/v2/rum/events"

	IntakeRUMV3Path = "/intake/v3/rum/events"

	// OTLPTracesIntakePath defines the path to ingest OpenTelemetry traces (HTTP Collector)
	OTLPTracesIntakePath = "/v1/traces"
	// OTLPMetricsIntakePath defines the path to ingest OpenTelemetry metrics (HTTP Collector)
	OTLPMetricsIntakePath = "/v1/metrics"
	// OTLPLogsIntakePath defines the path to ingest OpenTelemetry logs (HTTP Collector)
	OTLPLogsIntakePath = "/v1/logs"
)

// NewMux creates a new gorilla/mux router, with routes registered for handling the
// APM Server API.
func NewMux(
	beaterConfig *config.Config,
	batchProcessor modelpb.BatchProcessor,
	authenticator *auth.Authenticator,
	fetcher agentcfg.Fetcher,
	ratelimitStore *ratelimit.Store,
	sourcemapFetcher sourcemap.Fetcher,
	publishReady func() bool,
	semaphore input.Semaphore,
) (*mux.Router, error) {
	pool := request.NewContextPool()
	logger := logp.NewLogger(logs.Handler)
	router := mux.NewRouter()
	router.NotFoundHandler = pool.HTTPHandler(notFoundHandler)

	builder := routeBuilder{
		cfg:              beaterConfig,
		authenticator:    authenticator,
		batchProcessor:   batchProcessor,
		ratelimitStore:   ratelimitStore,
		sourcemapFetcher: sourcemapFetcher,
		intakeSemaphore:  semaphore,
	}

	zapLogger := zap.New(logger.Core(), zap.WithCaller(true))
	builder.intakeProcessor = elasticapm.NewProcessor(elasticapm.Config{
		MaxEventSize: beaterConfig.MaxEventSize,
		Semaphore:    semaphore,
		Logger:       zapLogger,
	})

	type route struct {
		path      string
		handlerFn func() (request.Handler, error)
	}

	otlpHandlers := otlp.NewHTTPHandlers(zapLogger, batchProcessor, semaphore)
	rumIntakeHandler := builder.rumIntakeHandler()
	routeMap := []route{
		{RootPath, builder.rootHandler(publishReady)},
		{AgentConfigPath, builder.backendAgentConfigHandler(fetcher)},
		{AgentConfigRUMPath, builder.rumAgentConfigHandler(fetcher)},
		{IntakeRUMPath, rumIntakeHandler},
		{IntakeRUMV3Path, rumIntakeHandler},
		{IntakePath, builder.backendIntakeHandler},
		{OTLPTracesIntakePath, builder.otlpHandler(otlpHandlers.HandleTraces, otlp.HTTPTracesMonitoringMap)},
		{OTLPMetricsIntakePath, builder.otlpHandler(otlpHandlers.HandleMetrics, otlp.HTTPMetricsMonitoringMap)},
		{OTLPLogsIntakePath, builder.otlpHandler(otlpHandlers.HandleLogs, otlp.HTTPLogsMonitoringMap)},
	}

	for _, route := range routeMap {
		h, err := route.handlerFn()
		if err != nil {
			return nil, err
		}
		logger.Infof("Path %s added to request handler", route.path)
		router.Handle(route.path, pool.HTTPHandler(h))
	}
	if beaterConfig.Expvar.Enabled {
		path := beaterConfig.Expvar.URL
		logger.Infof("Path %s added to request handler", path)
		router.Handle(path, http.HandlerFunc(debugVarsHandler))
	}
	if beaterConfig.Pprof.Enabled {
		const path = "/debug/pprof"
		logger.Infof("Path %s added to request handler", path)

		pprofRouter := router.PathPrefix(path).Subrouter().StrictSlash(true)
		pprofRouter.Handle("/", http.HandlerFunc(httppprof.Index))
		for _, p := range pprof.Profiles() {
			pprofRouter.Handle("/"+p.Name(), http.HandlerFunc(httppprof.Index))
		}
		pprofRouter.Handle("/cmdline", http.HandlerFunc(httppprof.Cmdline))
		pprofRouter.Handle("/profile", http.HandlerFunc(httppprof.Profile))
		pprofRouter.Handle("/symbol", http.HandlerFunc(httppprof.Symbol))
		pprofRouter.Handle("/trace", http.HandlerFunc(httppprof.Trace))
	}
	return router, nil
}

type routeBuilder struct {
	cfg              *config.Config
	authenticator    *auth.Authenticator
	batchProcessor   modelpb.BatchProcessor
	ratelimitStore   *ratelimit.Store
	sourcemapFetcher sourcemap.Fetcher
	intakeProcessor  *elasticapm.Processor
	intakeSemaphore  input.Semaphore
}

func (r *routeBuilder) backendIntakeHandler() (request.Handler, error) {
	h := intake.Handler(r.intakeProcessor, backendRequestMetadataFunc(r.cfg), r.batchProcessor)
	return middleware.Wrap(h, backendMiddleware(r.cfg, r.authenticator, r.ratelimitStore, intake.MonitoringMap)...)
}

func (r *routeBuilder) otlpHandler(handler http.HandlerFunc, monitoringMap map[request.ResultID]*monitoring.Int) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		h := func(c *request.Context) {
			handler(c.ResponseWriter, c.Request)
		}
		return middleware.Wrap(h, backendMiddleware(r.cfg, r.authenticator, r.ratelimitStore, monitoringMap)...)
	}
}

func (r *routeBuilder) rumIntakeHandler() func() (request.Handler, error) {
	return func() (request.Handler, error) {
		var batchProcessors modelprocessor.Chained
		// The order of these processors is important. Source mapping must happen before identifying library frames, or
		// frames to exclude from error grouping; identifying library frames must happen before updating the error culprit.
		if r.sourcemapFetcher != nil {
			batchProcessors = append(batchProcessors, sourcemap.BatchProcessor{
				Fetcher: r.sourcemapFetcher,
				Timeout: r.cfg.RumConfig.SourceMapping.Timeout,
				Logger:  logp.NewLogger(logs.Stacktrace),
			})
		}
		if r.cfg.RumConfig.LibraryPattern != "" {
			re, err := regexp.Compile(r.cfg.RumConfig.LibraryPattern)
			if err != nil {
				return nil, errors.Wrap(err, "invalid library pattern regex")
			}
			batchProcessors = append(batchProcessors, srvmodelprocessor.SetLibraryFrame{Pattern: re})
		}
		if r.cfg.RumConfig.ExcludeFromGrouping != "" {
			re, err := regexp.Compile(r.cfg.RumConfig.ExcludeFromGrouping)
			if err != nil {
				return nil, errors.Wrap(err, "invalid exclude from grouping regex")
			}
			batchProcessors = append(batchProcessors, srvmodelprocessor.SetExcludeFromGrouping{Pattern: re})
		}
		if r.sourcemapFetcher != nil {
			batchProcessors = append(batchProcessors, modelprocessor.SetCulprit{})
		}
		batchProcessors = append(batchProcessors, r.batchProcessor) // r.batchProcessor always goes last
		h := intake.Handler(r.intakeProcessor, rumRequestMetadataFunc(r.cfg), batchProcessors)
		return middleware.Wrap(h, rumMiddleware(r.cfg, r.authenticator, r.ratelimitStore, intake.MonitoringMap)...)
	}
}

func (r *routeBuilder) rootHandler(publishReady func() bool) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		h := root.Handler(root.HandlerConfig{
			Version:      version.Version,
			PublishReady: publishReady,
		})
		return middleware.Wrap(h, rootMiddleware(r.cfg, r.authenticator)...)
	}
}

func (r *routeBuilder) backendAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, r.authenticator, r.ratelimitStore, backendMiddleware, f)
	}
}

func (r *routeBuilder) rumAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, r.authenticator, r.ratelimitStore, rumMiddleware, f)
	}
}

type middlewareFunc func(*config.Config, *auth.Authenticator, *ratelimit.Store, map[request.ResultID]*monitoring.Int) []middleware.Middleware

func agentConfigHandler(
	cfg *config.Config,
	authenticator *auth.Authenticator,
	ratelimitStore *ratelimit.Store,
	middlewareFunc middlewareFunc,
	f agentcfg.Fetcher,
) (request.Handler, error) {
	mw := middlewareFunc(cfg, authenticator, ratelimitStore, agent.MonitoringMap)
	h := agent.NewHandler(f, cfg.AgentConfig.Cache.Expiration, cfg.DefaultServiceEnvironment, cfg.AgentAuth.Anonymous.AllowAgent)
	return middleware.Wrap(h, mw...)
}

func apmMiddleware(m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.LogMiddleware(),
		middleware.TimeoutMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(m),
	}
}

func backendMiddleware(cfg *config.Config, authenticator *auth.Authenticator, ratelimitStore *ratelimit.Store, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	backendMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthMiddleware(authenticator, true),
		middleware.AnonymousRateLimitMiddleware(ratelimitStore),
	)
	return backendMiddleware
}

func rumMiddleware(cfg *config.Config, authenticator *auth.Authenticator, ratelimitStore *ratelimit.Store, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	msg := "RUM endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable ingestion of RUM events. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	rumMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.ResponseHeadersMiddleware(cfg.RumConfig.ResponseHeaders),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins, cfg.RumConfig.AllowHeaders),
		middleware.AuthMiddleware(authenticator, true),
		middleware.AnonymousRateLimitMiddleware(ratelimitStore),
	)
	return append(rumMiddleware, middleware.KillSwitchMiddleware(cfg.RumConfig.Enabled, msg))
}

func rootMiddleware(cfg *config.Config, authenticator *auth.Authenticator) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthMiddleware(authenticator, false),
	)
}

func baseRequestMetadata(c *request.Context) *modelpb.APMEvent {
	return &modelpb.APMEvent{
		Timestamp: timestamppb.New(c.Timestamp),
	}
}

func backendRequestMetadataFunc(cfg *config.Config) func(c *request.Context) *modelpb.APMEvent {
	if !cfg.AugmentEnabled {
		return baseRequestMetadata
	}
	return func(c *request.Context) *modelpb.APMEvent {
		e := modelpb.APMEvent{
			Timestamp: timestamppb.New(c.Timestamp),
		}

		if c.ClientIP.IsValid() {
			e.Host = &modelpb.Host{
				Ip: []*modelpb.IP{modelpb.Addr2IP(c.ClientIP)},
			}
		}
		return &e
	}
}

func rumRequestMetadataFunc(cfg *config.Config) func(c *request.Context) *modelpb.APMEvent {
	if !cfg.AugmentEnabled {
		return baseRequestMetadata
	}
	return func(c *request.Context) *modelpb.APMEvent {
		e := modelpb.APMEvent{
			Timestamp: timestamppb.New(c.Timestamp),
		}
		if c.UserAgent != "" {
			e.UserAgent = &modelpb.UserAgent{Original: c.UserAgent}
		}
		if c.ClientIP.IsValid() {
			e.Client = &modelpb.Client{Ip: modelpb.Addr2IP(c.ClientIP)}
		}
		if c.SourcePort != 0 || c.SourceIP.IsValid() {
			e.Source = &modelpb.Source{Port: uint32(c.SourcePort)}
			if c.SourceIP.IsValid() {
				e.Source.Ip = modelpb.Addr2IP(c.SourceIP)
			}
		}
		if c.SourceNATIP.IsValid() {
			e.Source.Nat = &modelpb.NAT{Ip: modelpb.Addr2IP(c.SourceNATIP)}
		}
		return &e
	}
}

func notFoundHandler(c *request.Context) {
	c.Result.SetDefault(request.IDResponseErrorsNotFound)
	c.WriteResult()
}
