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

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/profile"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
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

type route struct {
	path      string
	handlerFn func(*config.Config, *authorization.Builder, publish.Reporter) (request.Handler, error)
}

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(beaterConfig *config.Config, report publish.Reporter) (*http.ServeMux, error) {
	pool := request.NewContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	auth, err := authorization.NewBuilder(beaterConfig)
	if err != nil {
		return nil, err
	}

	routeMap := []route{
		{RootPath, rootHandler},
		{AssetSourcemapPath, sourcemapHandler},
		{AgentConfigPath, backendAgentConfigHandler},
		{AgentConfigRUMPath, rumAgentConfigHandler},
		{IntakeRUMPath, rumIntakeHandler},
		{IntakeRUMV3Path, rumV3IntakeHandler},
		{IntakePath, backendIntakeHandler},
		// The profile endpoint is in Beta
		{ProfilePath, profileHandler},
	}

	for _, route := range routeMap {
		h, err := route.handlerFn(beaterConfig, auth, report)
		if err != nil {
			return nil, err
		}
		logger.Infof("Path %s added to request handler", route.path)
		mux.Handle(route.path, pool.HTTPHandler(h))

	}
	if beaterConfig.Expvar.IsEnabled() {
		path := beaterConfig.Expvar.URL
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, http.HandlerFunc(debugVarsHandler))
	}
	return mux, nil
}

func profileHandler(cfg *config.Config, builder *authorization.Builder, reporter publish.Reporter) (request.Handler, error) {
	h := profile.Handler(reporter)
	authHandler := builder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(cfg, authHandler, profile.MonitoringMap)...)
}

func backendIntakeHandler(cfg *config.Config, builder *authorization.Builder, reporter publish.Reporter) (request.Handler, error) {
	h := intake.Handler(stream.BackendProcessor(cfg), reporter)
	authHandler := builder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return middleware.Wrap(h, backendMiddleware(cfg, authHandler, intake.MonitoringMap)...)
}

func rumIntakeHandler(cfg *config.Config, _ *authorization.Builder, reporter publish.Reporter) (request.Handler, error) {
	h := intake.Handler(stream.RUMV2Processor(cfg), reporter)
	return middleware.Wrap(h, rumMiddleware(cfg, nil, intake.MonitoringMap)...)
}

func rumV3IntakeHandler(cfg *config.Config, _ *authorization.Builder, reporter publish.Reporter) (request.Handler, error) {
	h := intake.Handler(stream.RUMV3Processor(cfg), reporter)
	return middleware.Wrap(h, rumMiddleware(cfg, nil, intake.MonitoringMap)...)
}

func sourcemapHandler(cfg *config.Config, builder *authorization.Builder, reporter publish.Reporter) (request.Handler, error) {
	h := sourcemap.Handler(reporter)
	authHandler := builder.ForPrivilege(authorization.PrivilegeSourcemapWrite.Action)
	return middleware.Wrap(h, sourcemapMiddleware(cfg, authHandler)...)
}

func backendAgentConfigHandler(cfg *config.Config, builder *authorization.Builder, _ publish.Reporter) (request.Handler, error) {
	authHandler := builder.ForPrivilege(authorization.PrivilegeAgentConfigRead.Action)
	return agentConfigHandler(cfg, authHandler, backendMiddleware)
}

func rumAgentConfigHandler(cfg *config.Config, _ *authorization.Builder, _ publish.Reporter) (request.Handler, error) {
	return agentConfigHandler(cfg, nil, rumMiddleware)
}

type middlewareFunc func(*config.Config, *authorization.Handler, map[request.ResultID]*monitoring.Int) []middleware.Middleware

func agentConfigHandler(cfg *config.Config, authHandler *authorization.Handler, middlewareFunc middlewareFunc) (request.Handler, error) {
	var client kibana.Client
	if cfg.Kibana.Enabled {
		client = kibana.NewConnectingClient(&cfg.Kibana.ClientConfig)
	}
	h := agent.Handler(client, cfg.AgentConfig)
	msg := "Agent remote configuration is disabled. " +
		"Configure the `apm-server.kibana` section in apm-server.yml to enable it. " +
		"If you are using a RUM agent, you also need to configure the `apm-server.rum` section. " +
		"If you are not using remote configuration, you can safely ignore this error."
	ks := middleware.KillSwitchMiddleware(cfg.Kibana.Enabled, msg)
	return middleware.Wrap(h, append(middlewareFunc(cfg, authHandler, agent.MonitoringMap), ks)...)
}

func rootHandler(cfg *config.Config, builder *authorization.Builder, _ publish.Reporter) (request.Handler, error) {
	return middleware.Wrap(root.Handler(),
		rootMiddleware(cfg, builder.ForAnyOfPrivileges(authorization.ActionAny))...)
}

func apmMiddleware(m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.LogMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(m),
		middleware.RequestTimeMiddleware(),
	}
}

func backendMiddleware(cfg *config.Config, auth *authorization.Handler, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	backendMiddleware := append(apmMiddleware(m),
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
		middleware.ResponseHeadersMiddleware(cfg.RumConfig.ResponseHeaders),
		middleware.SetRumFlagMiddleware(),
		middleware.SetIPRateLimitMiddleware(cfg.RumConfig.EventRate),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins, cfg.RumConfig.AllowHeaders),
		middleware.KillSwitchMiddleware(cfg.RumConfig.IsEnabled(), msg),
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
	enabled := cfg.RumConfig.IsEnabled() && cfg.RumConfig.SourceMapping.IsEnabled()
	return append(backendMiddleware(cfg, auth, sourcemap.MonitoringMap),
		middleware.KillSwitchMiddleware(enabled, msg))
}

func rootMiddleware(_ *config.Config, auth *authorization.Handler) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.AuthorizationMiddleware(auth, false))
}
