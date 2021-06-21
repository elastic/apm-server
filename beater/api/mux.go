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
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/profile"
	"github.com/elastic/apm-server/beater/api/ratelimit"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
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

var (
	// rumAgents holds the current and previous agent names for the
	// RUM JavaScript agent. This is used for restricting which config
	// is supplied to anonymous agents.
	rumAgents = []string{"rum-js", "js-base"}
)

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(
	beatInfo beat.Info,
	beaterConfig *config.Config,
	report publish.Reporter,
	batchProcessor model.BatchProcessor,
	fetcher agentcfg.Fetcher,
	ratelimitStore *ratelimit.Store,
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
		ratelimitStore: ratelimitStore,
	}

	type route struct {
		path      string
		handlerFn func() (request.Handler, error)
	}
	routeMap := []route{
		{RootPath, builder.rootHandler},
		{AssetSourcemapPath, builder.sourcemapHandler},
		{AgentConfigPath, builder.backendAgentConfigHandler(fetcher)},
		{AgentConfigRUMPath, builder.rumAgentConfigHandler(fetcher)},
		{IntakeRUMPath, builder.rumIntakeHandler(stream.RUMV2Processor)},
		{IntakeRUMV3Path, builder.rumIntakeHandler(stream.RUMV3Processor)},
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
	ratelimitStore *ratelimit.Store
}

func (r *routeBuilder) profileHandler() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = backendRequestMetadata
	}
	h := profile.Handler(requestMetadataFunc, r.batchProcessor)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(r.cfg, authHandler, r.ratelimitStore, profile.MonitoringMap)...)
}

func (r *routeBuilder) backendIntakeHandler() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = backendRequestMetadata
	}
	h := intake.Handler(stream.BackendProcessor(r.cfg), requestMetadataFunc, r.batchProcessor)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(r.cfg, authHandler, r.ratelimitStore, intake.MonitoringMap)...)
}

func (r *routeBuilder) rumIntakeHandler(newProcessor func(*config.Config) *stream.Processor) func() (request.Handler, error) {
	requestMetadataFunc := emptyRequestMetadata
	if r.cfg.AugmentEnabled {
		requestMetadataFunc = rumRequestMetadata
	}
	return func() (request.Handler, error) {
		batchProcessor := r.batchProcessor
		batchProcessor = batchProcessorWithAllowedServiceNames(batchProcessor, r.cfg.RumConfig.AllowServiceNames)
		h := intake.Handler(newProcessor(r.cfg), requestMetadataFunc, batchProcessor)
		return middleware.Wrap(h, rumMiddleware(r.cfg, nil, r.ratelimitStore, intake.MonitoringMap)...)
	}
}

func (r *routeBuilder) sourcemapHandler() (request.Handler, error) {
	h := sourcemap.Handler(r.reporter)
	authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeSourcemapWrite.Action)
	return middleware.Wrap(h, sourcemapMiddleware(r.cfg, authHandler, r.ratelimitStore)...)
}

func (r *routeBuilder) rootHandler() (request.Handler, error) {
	h := root.Handler(root.HandlerConfig{Version: r.info.Version})
	return middleware.Wrap(h, rootMiddleware(r.cfg, r.authBuilder.ForAnyOfPrivileges(authorization.ActionAny))...)
}

func (r *routeBuilder) backendAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		authHandler := r.authBuilder.ForPrivilege(authorization.PrivilegeAgentConfigRead.Action)
		return agentConfigHandler(r.cfg, authHandler, r.ratelimitStore, backendMiddleware, f)
	}
}

func (r *routeBuilder) rumAgentConfigHandler(f agentcfg.Fetcher) func() (request.Handler, error) {
	return func() (request.Handler, error) {
		return agentConfigHandler(r.cfg, nil, r.ratelimitStore, rumMiddleware, f)
	}
}

type middlewareFunc func(*config.Config, *authorization.Handler, *ratelimit.Store, map[request.ResultID]*monitoring.Int) []middleware.Middleware

func agentConfigHandler(
	cfg *config.Config,
	authHandler *authorization.Handler,
	ratelimitStore *ratelimit.Store,
	middlewareFunc middlewareFunc,
	f agentcfg.Fetcher,
) (request.Handler, error) {
	mw := middlewareFunc(cfg, authHandler, ratelimitStore, agent.MonitoringMap)
	h := agent.NewHandler(f, cfg.KibanaAgentConfig, cfg.DefaultServiceEnvironment, rumAgents)

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

func backendMiddleware(cfg *config.Config, auth *authorization.Handler, ratelimitStore *ratelimit.Store, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	backendMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthorizationMiddleware(auth, true),
		middleware.AnonymousRateLimitMiddleware(ratelimitStore),
	)
	return backendMiddleware
}

func rumMiddleware(cfg *config.Config, auth *authorization.Handler, ratelimitStore *ratelimit.Store, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	msg := "RUM endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable ingestion of RUM events. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	rumMiddleware := append(apmMiddleware(m),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.ResponseHeadersMiddleware(cfg.RumConfig.ResponseHeaders),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins, cfg.RumConfig.AllowHeaders),
		middleware.AnonymousAuthorizationMiddleware(),
		middleware.AnonymousRateLimitMiddleware(ratelimitStore),
	)
	return append(rumMiddleware, middleware.KillSwitchMiddleware(cfg.RumConfig.Enabled, msg))
}

func sourcemapMiddleware(cfg *config.Config, auth *authorization.Handler, ratelimitStore *ratelimit.Store) []middleware.Middleware {
	msg := "Sourcemap upload endpoint is disabled. " +
		"Configure the `apm-server.rum` section in apm-server.yml to enable sourcemap uploads. " +
		"If you are not using the RUM agent, you can safely ignore this error."
	if cfg.DataStreams.Enabled {
		msg = "When APM Server is managed by Fleet, Sourcemaps must be uploaded directly to Elasticsearch."
	}
	enabled := cfg.RumConfig.Enabled && cfg.RumConfig.SourceMapping.Enabled && !cfg.DataStreams.Enabled
	backendMiddleware := backendMiddleware(cfg, auth, ratelimitStore, sourcemap.MonitoringMap)
	return append(backendMiddleware, middleware.KillSwitchMiddleware(enabled, msg))
}

func rootMiddleware(cfg *config.Config, auth *authorization.Handler) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.ResponseHeadersMiddleware(cfg.ResponseHeaders),
		middleware.AuthorizationMiddleware(auth, false))
}

// TODO(axw) move this into the authorization package when introducing anonymous auth.
func batchProcessorWithAllowedServiceNames(p model.BatchProcessor, allowedServiceNames []string) model.BatchProcessor {
	if len(allowedServiceNames) == 0 {
		// All service names are allowed.
		return p
	}
	m := make(map[string]bool)
	for _, name := range allowedServiceNames {
		m[name] = true
	}
	var restrictServiceName modelprocessor.MetadataProcessorFunc = func(ctx context.Context, meta *model.Metadata) error {
		// Restrict to explicitly allowed service names.
		// The list of allowed service names is not considered secret,
		// so we do not use constant time comparison.
		if !m[meta.Service.Name] {
			return &stream.InvalidInputError{Message: "service name is not allowed"}
		}
		return nil
	}
	return modelprocessor.Chained{restrictServiceName, p}
}

func emptyRequestMetadata(c *request.Context) model.Metadata {
	return model.Metadata{}
}

func backendRequestMetadata(c *request.Context) model.Metadata {
	return model.Metadata{System: model.System{IP: c.SourceIP}}
}

func rumRequestMetadata(c *request.Context) model.Metadata {
	return model.Metadata{
		Client:    model.Client{IP: c.SourceIP},
		UserAgent: model.UserAgent{Original: c.UserAgent},
	}
}
