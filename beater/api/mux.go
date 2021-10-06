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
	"net"
	"net/http"
	"net/http/pprof"
	"regexp"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	apisourcemap "github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/firehose"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/profile"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/beater/request"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sourcemap"
)

const (
	// RootPath defines the server's root path
	RootPath = "/"

	// Backend routes

	// AgentConfigPath defines the path to query for agent config management
	AgentConfigPath = "/config/v1/agents"
	// AssetSourcemapPath defines the path to upload sourcemaps
	AssetSourcemapPath = "/assets/v1/sourcemaps"
	// IntakePath defines the path to ingest monitored events
	IntakePath = "/intake/v2/events"
	// ProfilePath defines the path to ingest profiles
	ProfilePath = "/intake/v2/profile"

	// RUM routes

	// AgentConfigRUMPath defines the path to query for the RUM agent config management
	AgentConfigRUMPath = "/config/v1/rum/agents"
	// IntakeRUMPath defines the path to ingest monitored RUM events
	IntakeRUMPath = "/intake/v2/rum/events"

	IntakeRUMV3Path = "/intake/v3/rum/events"

	// FirehosePath defines the path to ingest firehose logs
	FirehosePath = "/firehose"
)

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(
	beatInfo beat.Info,
	beaterConfig *config.Config,
	report publish.Reporter,
	batchProcessor model.BatchProcessor,
	authenticator *auth.Authenticator,
	fetcher agentcfg.Fetcher,
	ratelimitStore *ratelimit.Store,
	sourcemapStore *sourcemap.Store,
	fleetManaged bool,
	publishReady func() bool,
) (*http.ServeMux, error) {
	pool := request.NewContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	builder := routeBuilder{
		info:           beatInfo,
		cfg:            beaterConfig,
		authenticator:  authenticator,
		reporter:       report,
		batchProcessor: batchProcessor,
		ratelimitStore: ratelimitStore,
		sourcemapStore: sourcemapStore,
		fleetManaged:   fleetManaged,
	}

	type route struct {
		path      string
		handlerFn func() (request.Handler, error)
	}
	routeMap := []route{
		{RootPath, builder.rootHandler(publishReady)},
		{AssetSourcemapPath, builder.sourcemapHandler},
		{AgentConfigPath, builder.backendAgentConfigHandler(fetcher)},
		{AgentConfigRUMPath, builder.rumAgentConfigHandler(fetcher)},
		{IntakeRUMPath, builder.rumIntakeHandler(stream.RUMV2Processor)},
		{IntakeRUMV3Path, builder.rumIntakeHandler(stream.RUMV3Processor)},
		{IntakePath, builder.backendIntakeHandler},
		// The profile endpoint is in Beta
		{ProfilePath, builder.profileHandler},
		// The firehose endpoint is experimental and subject to breaking changes and removal.
		{FirehosePath, builder.firehoseLogHandler},
	}

	for _, route := range routeMap {
		h, err := route.handlerFn()
		if err != nil {
			return nil, err
		}
		logger.Infof("Path %s added to request handler", route.path)
		mux.Handle(route.path, pool.HTTPHandler(h))
	}
	if beaterConfig.Expvar.Enabled {
		path := beaterConfig.Expvar.URL
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, http.HandlerFunc(debugVarsHandler))
	}
	if beaterConfig.Pprof.Enabled {
		const path = "/debug/pprof"
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path+"/", http.HandlerFunc(pprof.Index))
		mux.Handle(path+"/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle(path+"/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle(path+"/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle(path+"/trace", http.HandlerFunc(pprof.Trace))
	}
	return mux, nil
}

type routeBuilder struct {
	info           beat.Info
	cfg            *config.Config
	authenticator  *auth.Authenticator
	reporter       publish.Reporter
	batchProcessor model.BatchProcessor
	ratelimitStore *ratelimit.Store
	sourcemapStore *sourcemap.Store
	fleetManaged   bool
}

func (r *routeBuilder) profileHandler() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = backendRequestMetadata
	}
	h := profile.Handler(requestMetadataFunc, r.batchProcessor)
	return middleware.Wrap(h, backendMiddleware(r.cfg, r.authenticator, r.ratelimitStore, profile.MonitoringMap)...)
}

func (r *routeBuilder) firehoseLogHandler() (request.Handler, error) {
	h := firehose.Handler(r.batchProcessor, r.authenticator)
	return middleware.Wrap(h, firehoseMiddleware(r.cfg, intake.MonitoringMap)...)
}

func (r *routeBuilder) backendIntakeHandler() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = backendRequestMetadata
	}
	h := intake.Handler(stream.BackendProcessor(r.cfg), requestMetadataFunc, r.batchProcessor)
	return middleware.Wrap(h, backendMiddleware(r.cfg, r.authenticator, r.ratelimitStore, intake.MonitoringMap)...)
}

func (r *routeBuilder) rumIntakeHandler(newProcessor func(*config.Config) *stream.Processor) func() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = rumRequestMetadata
	}
	return func() (request.Handler, error) {
		var batchProcessors modelprocessor.Chained
		// The order of these processors is important. Source mapping must happen before identifying library frames, or
		// frames to exclude from error grouping; identifying library frames must happen before updating the error culprit.
		if r.sourcemapStore != nil {
			batchProcessors = append(batchProcessors, sourcemap.BatchProcessor{
				Store:   r.sourcemapStore,
				Timeout: r.cfg.RumConfig.SourceMapping.Timeout,
			})
		}
		if r.cfg.RumConfig.LibraryPattern != "" {
			re, err := regexp.Compile(r.cfg.RumConfig.LibraryPattern)
			if err != nil {
				return nil, errors.Wrap(err, "invalid library pattern regex")
			}
			batchProcessors = append(batchProcessors, modelprocessor.SetLibraryFrame{Pattern: re})
		}
		if r.cfg.RumConfig.ExcludeFromGrouping != "" {
			re, err := regexp.Compile(r.cfg.RumConfig.ExcludeFromGrouping)
			if err != nil {
				return nil, errors.Wrap(err, "invalid exclude from grouping regex")
			}
			batchProcessors = append(batchProcessors, modelprocessor.SetExcludeFromGrouping{Pattern: re})
		}
		if r.sourcemapStore != nil {
			batchProcessors = append(batchProcessors, modelprocessor.SetCulprit{})
		}
		batchProcessors = append(batchProcessors, r.batchProcessor) // r.batchProcessor always goes last
		h := intake.Handler(newProcessor(r.cfg), requestMetadataFunc, batchProcessors)
		return middleware.Wrap(h, rumMiddleware(r.cfg, r.authenticator, r.ratelimitStore, intake.MonitoringMap)...)
	}
}

func (r *routeBuilder) sourcemapHandler() (request.Handler, error) {
	h := apisourcemap.Handler(r.reporter, r.sourcemapStore)
	return middleware.Wrap(h, sourcemapMiddleware(r.cfg, r.authenticator, r.ratelimitStore)...)
}

func (r *routeBuilder) rootHandler(publishReady func() bool) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		h := root.Handler(root.HandlerConfig{
			Version:      r.info.Version,
			PublishReady: publishReady,
		})
		return middleware.Wrap(h, rootMiddleware(r.cfg, r.authenticator)...)
	}
}

func (r *routeBuilder) backendAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, r.authenticator, r.ratelimitStore, backendMiddleware, f, r.fleetManaged)
	}
}

func (r *routeBuilder) rumAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, r.authenticator, r.ratelimitStore, rumMiddleware, f, r.fleetManaged)
	}
}

type middlewareFunc func(*config.Config, *auth.Authenticator, *ratelimit.Store, map[request.ResultID]*monitoring.Int) []middleware.Middleware

func agentConfigHandler(
	cfg *config.Config,
	authenticator *auth.Authenticator,
	ratelimitStore *ratelimit.Store,
	middlewareFunc middlewareFunc,
	f agentcfg.Fetcher,
	fleetManaged bool,
) (request.Handler, error) {
	mw := middlewareFunc(cfg, authenticator, ratelimitStore, agent.MonitoringMap)
	h := agent.NewHandler(f, cfg.KibanaAgentConfig, cfg.DefaultServiceEnvironment, cfg.AgentAuth.Anonymous.AllowAgent)

	if !cfg.Kibana.Enabled && !fleetManaged {
		msg := "Agent remote configuration is disabled. " +
			"Configure the `apm-server.kibana` section in apm-server.yml to enable it. " +
			"If you are using a RUM agent, you also need to configure the `apm-server.rum` section. " +
			"If you are not using remote configuration, you can safely ignore this error."
		mw = append(mw, middleware.KillSwitchMiddleware(cfg.Kibana.Enabled, msg))
	}

	return middleware.Wrap(h, mw...)
}

func apmMiddleware(m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.LogMiddleware(),
		middleware.TimeoutMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(m),
		middleware.RequestTimeMiddleware(),
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

func sourcemapMiddleware(cfg *config.Config, auth *auth.Authenticator, ratelimitStore *ratelimit.Store) []middleware.Middleware {
	msg := "Sourcemap upload endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable sourcemap uploads. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	if cfg.DataStreams.Enabled {
		msg = "When APM Server is managed by Fleet, Sourcemaps must be uploaded directly to Elasticsearch."
	}
	enabled := cfg.RumConfig.Enabled && cfg.RumConfig.SourceMapping.Enabled && !cfg.DataStreams.Enabled
	backendMiddleware := backendMiddleware(cfg, auth, ratelimitStore, apisourcemap.MonitoringMap)
	return append(backendMiddleware, middleware.KillSwitchMiddleware(enabled, msg))
}

func rootMiddleware(cfg *config.Config, authenticator *auth.Authenticator) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthMiddleware(authenticator, false),
	)
}

func firehoseMiddleware(cfg *config.Config, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	firehoseMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
	)
	return firehoseMiddleware
}

func emptyRequestMetadata(c *request.Context) model.APMEvent {
	return model.APMEvent{}
}

func backendRequestMetadata(c *request.Context) model.APMEvent {
	return model.APMEvent{Host: model.Host{
		IP: c.ClientIP,
	}}
}

func rumRequestMetadata(c *request.Context) model.APMEvent {
	var source model.Source
	if tcpAddr, ok := c.SourceAddr.(*net.TCPAddr); ok {
		source.IP = tcpAddr.IP
		source.Port = tcpAddr.Port
	}
	return model.APMEvent{
		Client:    model.Client{IP: c.ClientIP},
		Source:    source,
		UserAgent: model.UserAgent{Original: c.UserAgent},
	}
}
