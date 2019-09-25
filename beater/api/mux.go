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

	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	psourcemap "github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

const (
	// RootPath defines the server's root path
	RootPath = "/"

	// AgentConfigPath defines the path to query for agent config management
	AgentConfigPath = "/config/v1/agents"

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
	handlerFn func(*config.Config, publish.Reporter) (request.Handler, error)
}

// NewMux registers apm handlers to paths building up the APM Server API.
func NewMux(beaterConfig *config.Config, report publish.Reporter) (*http.ServeMux, error) {
	pool := newContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	routeMap := []route{
		{RootPath, rootHandler},
		{AssetSourcemapPath, sourcemapHandler},
		{AgentConfigPath, backendAgentConfigHandler},
		{IntakeRUMPath, rumHandler},
		{IntakePath, backendHandler},
	}

	for _, route := range routeMap {
		h, err := route.handlerFn(beaterConfig, report)
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

func backendHandler(cfg *config.Config, reporter publish.Reporter) (request.Handler, error) {
	h := intake.Handler(systemMetadataDecoder(cfg, emptyDecoder),
		&stream.Processor{
			Tconfig:      transform.Config{},
			Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		},
		reporter)
	return middleware.Wrap(h, backendMiddleware(cfg, intake.MonitoringMap)...)
}

func rumHandler(cfg *config.Config, reporter publish.Reporter) (request.Handler, error) {
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

func sourcemapHandler(cfg *config.Config, reporter publish.Reporter) (request.Handler, error) {
	tcfg, err := rumTransformConfig(cfg)
	if err != nil {
		return nil, err
	}
	h := sourcemap.Handler(systemMetadataDecoder(cfg, decoder.DecodeSourcemapFormData), psourcemap.Processor, *tcfg, reporter)
	return middleware.Wrap(h, sourcemapMiddleware(cfg)...)
}

func backendAgentConfigHandler(cfg *config.Config, _ publish.Reporter) (request.Handler, error) {
	return agentConfigHandler(cfg, backendMiddleware(cfg, agent.MonitoringMap))
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

func rootHandler(cfg *config.Config, _ publish.Reporter) (request.Handler, error) {
	return middleware.Wrap(root.Handler(), rootMiddleware(cfg)...)
}

func apmMiddleware(m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return []middleware.Middleware{
		middleware.LogMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(m),
		middleware.RequestTimeMiddleware(),
	}
}

func backendMiddleware(cfg *config.Config, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return append(apmMiddleware(m),
		middleware.RequireAuthorizationMiddleware(cfg.SecretToken))
}

func rumMiddleware(cfg *config.Config, m map[request.ResultID]*monitoring.Int) []middleware.Middleware {
	return append(apmMiddleware(m),
		middleware.SetRateLimitMiddleware(cfg.RumConfig.EventRate),
		middleware.CORSMiddleware(cfg.RumConfig.AllowOrigins),
		middleware.KillSwitchMiddleware(cfg.RumConfig.IsEnabled()))
}

func sourcemapMiddleware(cfg *config.Config) []middleware.Middleware {
	enabled := cfg.RumConfig.IsEnabled() && cfg.RumConfig.SourceMapping.IsEnabled()
	return append(backendMiddleware(cfg, sourcemap.MonitoringMap),
		middleware.KillSwitchMiddleware(enabled))
}

func rootMiddleware(cfg *config.Config) []middleware.Middleware {
	return append(apmMiddleware(root.MonitoringMap),
		middleware.SetAuthorizationMiddleware(cfg.SecretToken))
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
