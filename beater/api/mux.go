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
	"expvar"
	"net/http"
	"regexp"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/middleware/authorization"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	psourcemap "github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

const (
	// RootPath defines the server's root path
	RootPath = "/"

	// AgentConfigPath defines the path to query for agent config management
	AgentConfigPath = "/config/v1/agents"

	// AgentConfigRUMPath defines the path to query for the RUM agent config management
	AgentConfigRUMPath = "/config/v1/rum/agents"

	// IntakePath defines the path to ingest monitored events
	IntakePath = "/intake/v2/events"
	// IntakeRUMPath defines the path to ingest monitored RUM events
	IntakeRUMPath = "/intake/v2/rum/events"

	// AssetSourcemapPath defines the path to upload sourcemaps
	AssetSourcemapPath = "/assets/v1/sourcemaps"
)

var (
	emptyDecoder = func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
)

type route struct {
	path      string
	authMeans middleware.AuthMeans
	handlerFn func(*config.Config, middleware.AuthMeans, publish.Reporter) (request.Handler, error)
}

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(beaterConfig *config.Config, report publish.Reporter) (*http.ServeMux, error) {
	pool := newContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	backendAuthMeans, err := backendAuthMeans(beaterConfig)
	if err != nil {
		return nil, err
	}

	routeMap := []route{
		{RootPath, backendAuthMeans, rootHandler},

		{AssetSourcemapPath, backendAuthMeans, sourcemapHandler},
		{AgentConfigPath, backendAuthMeans, backendAgentConfigHandler},
		{IntakePath, backendAuthMeans, backendIntakeHandler},

		{AgentConfigRUMPath, nil, rumAgentConfigHandler},
		{IntakeRUMPath, nil, rumIntakeHandler},
	}

	for _, route := range routeMap {
		h, err := route.handlerFn(beaterConfig, route.authMeans, report)
		if err != nil {
			return nil, err
		}
		logger.Infof("Path %s added to request handler", route.path)
		mux.Handle(route.path, pool.handler(h))

	}
	if beaterConfig.Expvar.IsEnabled() {
		path := beaterConfig.Expvar.URL
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux, nil
}

func backendIntakeHandler(cfg *config.Config, means middleware.AuthMeans, reporter publish.Reporter) (request.Handler, error) {
	h := intake.Handler(systemMetadataDecoder(cfg, emptyDecoder),
		&stream.Processor{
			Tconfig:      transform.Config{},
			Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		},
		reporter)
	return middleware.Wrap(h, backendMiddleware(means, authorization.PrivilegeAccess, intake.MonitoringMap)...)
}

func rumIntakeHandler(cfg *config.Config, _ middleware.AuthMeans, reporter publish.Reporter) (request.Handler, error) {
	tcfg, err := rumTransformConfig(cfg)
	if err != nil {
		return nil, err
	}
	h := intake.Handler(userMetaDataDecoder(cfg, emptyDecoder),
		&stream.Processor{
			Tconfig:      *tcfg,
			Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		},
		reporter)
	return middleware.Wrap(h, rumMiddleware(cfg, intake.MonitoringMap)...)
}

func sourcemapHandler(cfg *config.Config, means middleware.AuthMeans, reporter publish.Reporter) (request.Handler, error) {
	tcfg, err := rumTransformConfig(cfg)
	if err != nil {
		return nil, err
	}
	h := sourcemap.Handler(systemMetadataDecoder(cfg, decoder.DecodeSourcemapFormData), psourcemap.Processor, *tcfg, reporter)
	return middleware.Wrap(h, sourcemapMiddleware(cfg, means)...)
}

func backendAgentConfigHandler(cfg *config.Config, means middleware.AuthMeans, _ publish.Reporter) (request.Handler, error) {
	return agentConfigHandler(cfg, backendMiddleware(means, authorization.PrivilegeAccess, agent.MonitoringMap))
}

func rumAgentConfigHandler(cfg *config.Config, _ middleware.AuthMeans, _ publish.Reporter) (request.Handler, error) {
	return agentConfigHandler(cfg, rumMiddleware(cfg, agent.MonitoringMap))
}

func agentConfigHandler(cfg *config.Config, m []middleware.Middleware) (request.Handler, error) {
	var kbClient kibana.Client
	if cfg.Kibana.Enabled() {
		kbClient = kibana.NewConnectingClient(cfg.Kibana)
	}
	h := agent.Handler(kbClient, cfg.AgentConfig)
	ks := middleware.KillSwitchMiddleware(cfg.Kibana.Enabled())
	return middleware.Wrap(h, append(m, ks)...)
}

func rootHandler(cfg *config.Config, means middleware.AuthMeans, _ publish.Reporter) (request.Handler, error) {
	return middleware.Wrap(root.Handler(), rootMiddleware(means)...)
}

func apmMiddleware(m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.LogMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(m),
		middleware.RequestTimeMiddleware(),
	}
}

func backendMiddleware(auth middleware.AuthMeans, privilege string, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return append(apmMiddleware(m),
		middleware.AuthorizationMiddleware(true, auth, privilege))
}

func rumMiddleware(cfg *config.Config, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return append(apmMiddleware(m),
		middleware.SetRumFlagMiddleware(),
		middleware.SetIPRateLimitMiddleware(cfg.RumConfig.EventRate),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins),
		middleware.KillSwitchMiddleware(cfg.RumConfig.IsEnabled()))
}

func sourcemapMiddleware(cfg *config.Config, auth middleware.AuthMeans) []middleware.Middleware {
	return append(apmMiddleware(sourcemap.MonitoringMap),
		middleware.KillSwitchMiddleware(cfg.RumConfig.IsEnabled() && cfg.RumConfig.SourceMapping.IsEnabled()),
		middleware.AuthorizationMiddleware(true, auth, authorization.PrivilegeAccess))
}

func rootMiddleware(auth middleware.AuthMeans) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.AuthorizationMiddleware(false, auth, authorization.PrivilegeAccess))
}

func systemMetadataDecoder(beaterConfig *config.Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeSystemData(d, beaterConfig.AugmentEnabled)
}

func userMetaDataDecoder(beaterConfig *config.Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeUserData(d, beaterConfig.AugmentEnabled)
}

func rumTransformConfig(beaterConfig *config.Config) (*transform.Config, error) {
	mapper, err := beaterConfig.RumConfig.MemoizedSourcemapMapper()
	if err != nil {
		return nil, err
	}
	return &transform.Config{
		SourcemapMapper:     mapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}, nil
}

func backendAuthMeans(cfg *config.Config) (middleware.AuthMeans, error) {
	auth := middleware.AuthMeans{}
	addAuthBearer(cfg, &auth)
	if err := addAuthAPIKey(cfg, &auth); err != nil {
		return nil, err
	}
	return auth, nil
}

func addAuthAPIKey(cfg *config.Config, authMeans *middleware.AuthMeans) error {
	if !cfg.AuthConfig.APIKeyConfig.IsEnabled() {
		return nil
	}
	client, err := elasticsearch.Client(cfg.AuthConfig.APIKeyConfig.ESConfig)
	if err != nil {
		return err
	}

	cache := authorization.NewAPIKeyCache(
		cfg.AuthConfig.APIKeyConfig.Cache.Expiration,
		cfg.AuthConfig.APIKeyConfig.Cache.Size,
		client)

	(*authMeans)[headers.APIKey] = func(token string) request.Authorization {
		return authorization.NewAPIKey(client, cache, token)
	}
	return nil
}

func addAuthBearer(cfg *config.Config, authMeans *middleware.AuthMeans) {
	if cfg.AuthConfig.BearerToken != "" {
		(*authMeans)[headers.Bearer] = func(token string) request.Authorization {
			return authorization.NewBearer(cfg.AuthConfig.BearerToken, token)
		}
	}
}
