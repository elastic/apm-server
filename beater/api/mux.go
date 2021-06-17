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
	"net/http/pprof"

	"go.elastic.co/apm"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/profile"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
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
)

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(
	beatInfo beat.Info,
	beaterConfig *config.Config,
	report publish.Reporter,
	batchProcessor model.BatchProcessor,
	fetcher agentcfg.Fetcher,
	tracer *apm.Tracer,
) (*http.ServeMux, error) {
	pool := request.NewContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	auth, err := authorization.NewBuilder(beaterConfig.AgentAuth)
	if err != nil {
		return nil, err
	}

	builder := routeBuilder{
		info:           beatInfo,
		cfg:            beaterConfig,
		authBuilder:    auth,
		reporter:       report,
		batchProcessor: batchProcessor,
	}

	type route struct {
		path      string
		handlerFn func() (request.Handler, error)
	}
	routeMap := []route{
		{RootPath, builder.rootHandler},
		{AssetSourcemapPath, builder.sourcemapHandler},
		{AgentConfigPath, builder.backendAgentConfigHandler(fetcher, tracer)},
		{AgentConfigRUMPath, builder.rumAgentConfigHandler(fetcher, tracer)},
		{IntakeRUMPath, builder.rumIntakeHandler},
		{IntakeRUMV3Path, builder.rumV3IntakeHandler},
		{IntakePath, builder.backendIntakeHandler},
		// The profile endpoint is in Beta
		{ProfilePath, builder.profileHandler},
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
	authBuilder    *authorization.Builder
	reporter       publish.Reporter
	batchProcessor model.BatchProcessor
}

func (r *routeBuilder) profileHandler() (request.Handler, error) {
	h := profile.Handler(r.batchProcessor)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(r.cfg, authHandler, profile.MonitoringMap)...)
}

func (r *routeBuilder) backendIntakeHandler() (request.Handler, error) {
	h := intake.Handler(stream.BackendProcessor(r.cfg), r.batchProcessor)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(r.cfg, authHandler, intake.MonitoringMap)...)
}

func (r *routeBuilder) rumIntakeHandler() (request.Handler, error) {
	h := intake.Handler(stream.RUMV2Processor(r.cfg), r.batchProcessor)
	return middleware.Wrap(h, rumMiddleware(r.cfg, nil, intake.MonitoringMap)...)
}

func (r *routeBuilder) rumV3IntakeHandler() (request.Handler, error) {
	h := intake.Handler(stream.RUMV3Processor(r.cfg), r.batchProcessor)
	return middleware.Wrap(h, rumMiddleware(r.cfg, nil, intake.MonitoringMap)...)
}

func (r *routeBuilder) sourcemapHandler() (request.Handler, error) {
	h := sourcemap.Handler(r.reporter)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeSourcemapWrite.Action)
	return middleware.Wrap(h, sourcemapMiddleware(r.cfg, authHandler)...)
}

func (r *routeBuilder) rootHandler() (request.Handler, error) {
	h := root.Handler(root.HandlerConfig{Version: r.info.Version})
	return middleware.Wrap(h, rootMiddleware(r.cfg, r.authBuilder.ForAnyOfPrivileges(authorization.ActionAny))...)
}

func (r *routeBuilder) backendAgentConfigHandler(f agentcfg.Fetcher, tracer *apm.Tracer) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeAgentConfigRead.Action)
		return agentConfigHandler(r.cfg, authHandler, backendMiddleware, f, tracer)
	}
}

func (r *routeBuilder) rumAgentConfigHandler(f agentcfg.Fetcher, tracer *apm.Tracer) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, nil, rumMiddleware, f, tracer)
	}
}

type middlewareFunc func(*config.Config, *authorization.Handler, map[request.ResultID]*monitoring.Int) []middleware.Middleware

func agentConfigHandler(
	cfg *config.Config,
	authHandler *authorization.Handler,
	middlewareFunc middlewareFunc,
	f agentcfg.Fetcher,
	tracer *apm.Tracer,
) (request.Handler, error) {
	mw := middlewareFunc(cfg, authHandler, agent.MonitoringMap)
	h := agent.NewHandler(f, cfg.KibanaAgentConfig, cfg.DefaultServiceEnvironment, tracer)

	if !cfg.Kibana.Enabled && cfg.AgentConfigs == nil {
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

func backendMiddleware(cfg *config.Config, auth *authorization.Handler, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	backendMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthorizationMiddleware(auth, true),
	)
	if cfg.AugmentEnabled {
		backendMiddleware = append(backendMiddleware, middleware.SystemMetadataMiddleware())
	}
	return backendMiddleware
}

func rumMiddleware(cfg *config.Config, _ *authorization.Handler, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	msg := "RUM endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable ingestion of RUM events. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	rumMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.ResponseHeadersMiddleware(cfg.RumConfig.ResponseHeaders),
		middleware.SetRumFlagMiddleware(),
		middleware.SetIPRateLimitMiddleware(cfg.RumConfig.EventRate),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins, cfg.RumConfig.AllowHeaders),
		middleware.AnonymousAuthorizationMiddleware(),
		middleware.KillSwitchMiddleware(cfg.RumConfig.Enabled, msg),
	)
	if cfg.AugmentEnabled {
		rumMiddleware = append(rumMiddleware, middleware.UserMetadataMiddleware())
	}
	return rumMiddleware
}

func sourcemapMiddleware(cfg *config.Config, auth *authorization.Handler) []middleware.Middleware {
	msg := "Sourcemap upload endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable sourcemap uploads. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	if cfg.DataStreams.Enabled {
		msg = "When APM Server is managed by Fleet, Sourcemaps must be uploaded directly to Elasticsearch."
	}
	enabled := cfg.RumConfig.Enabled && cfg.RumConfig.SourceMapping.Enabled && !cfg.DataStreams.Enabled
	return append(backendMiddleware(cfg, auth, sourcemap.MonitoringMap),
		middleware.KillSwitchMiddleware(enabled, msg))
}

func rootMiddleware(cfg *config.Config, auth *authorization.Handler) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthorizationMiddleware(auth, false))
}
