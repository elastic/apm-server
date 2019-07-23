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

package beater

import (
	"expvar"
	"net/http"
	"regexp"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

const (
	rootPath = "/"

	acmPath = "/config/v1/agents"

	// intake v2
	intakePath    = "/intake/v2/events"
	intakeRumPath = "/intake/v2/rum/events"

	// assets
	assetSourcemapPath = "/assets/v1/sourcemaps"

	burstMultiplier = 3
)

var (
	emptyDecoder = func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
)

type route struct {
	path string
	fn   func(*Config, publish.Reporter) (request.Handler, error)
}

func newMuxer(beaterConfig *Config, report publish.Reporter) (*http.ServeMux, error) {
	pool := newContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	routeMap := []route{
		{rootPath, defaultHandler},
		{assetSourcemapPath, sourcemapHandler},
		{acmPath, agentHandler},
		{intakeRumPath, rumHandler},
		{intakePath, backendHandler},
	}

	for _, route := range routeMap {
		h, err := route.fn(beaterConfig, report)
		if err != nil {
			return nil, err
		}
		logger.Infof("Path %s added to request handler", route.path)
		mux.Handle(route.path, pool.handler(h))

	}
	if beaterConfig.Expvar.isEnabled() {
		path := beaterConfig.Expvar.Url
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux, nil
}

func apmHandler(fn func(request.ResultID) *monitoring.Int) []middleware {
	return []middleware{
		logHandler(),
		monitoringHandler(fn),
		panicHandler(),
	}
}

func backendHandler(cfg *Config, reporter publish.Reporter) (request.Handler, error) {
	dec := systemMetadataDecoder(cfg, emptyDecoder)
	h := newIntakeHandler(dec,
		&stream.Processor{
			Tconfig:      transform.Config{},
			Mconfig:      model.Config{Experimental: cfg.Mode == ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		},
		nil,
		reporter)

	return withMiddleware(
		h,
		append(apmHandler(intakeResultIdToMonitoringInt),
			requestTimeHandler(),
			authHandler(cfg.SecretToken))...), nil
}

func rumHandler(cfg *Config, reporter publish.Reporter) (request.Handler, error) {
	dec := userMetaDataDecoder(cfg, emptyDecoder)

	tcfg, err := rumTransformConfig(cfg)
	if err != nil {
		return nil, err
	}

	cache, err := newRlCache(cfg.RumConfig.EventRate.LruSize, cfg.RumConfig.EventRate.Limit, burstMultiplier)
	if err != nil {
		return nil, err
	}
	h := newIntakeHandler(dec,
		&stream.Processor{
			Tconfig:      *tcfg,
			Mconfig:      model.Config{Experimental: cfg.Mode == ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		},
		cache,
		reporter)

	return withMiddleware(
		h,
		append(apmHandler(intakeResultIdToMonitoringInt),
			killSwitchHandler(cfg.RumConfig.isEnabled()),
			requestTimeHandler(),
			corsHandler(cfg.RumConfig.AllowOrigins))...), nil
}

//TODO: have dedicated monitoring function (breaking change)
func sourcemapHandler(cfg *Config, reporter publish.Reporter) (request.Handler, error) {
	tcfg, err := rumTransformConfig(cfg)
	if err != nil {
		return nil, err
	}
	dec := systemMetadataDecoder(cfg, decoder.DecodeSourcemapFormData)
	h := newAssetHandler(dec, sourcemap.Processor, *tcfg, reporter)

	return withMiddleware(
		h,
		append(apmHandler(intakeResultIdToMonitoringInt),
			killSwitchHandler(cfg.RumConfig.isEnabled() && cfg.RumConfig.SourceMapping.isEnabled()),
			authHandler(cfg.SecretToken))...), nil
}

func agentHandler(cfg *Config, _ publish.Reporter) (request.Handler, error) {
	var kbClient kibana.Client
	if cfg.Kibana.Enabled() {
		kbClient = kibana.NewConnectingClient(cfg.Kibana)
	}

	return withMiddleware(
		agentConfigHandler(kbClient, cfg.AgentConfig, cfg.SecretToken),
		append(apmHandler(acmResultIdToMonitoringInt),
			killSwitchHandler(kbClient != nil),
			authHandler(cfg.SecretToken))...), nil
}

//TODO: have dedicated monitoring function (breaking change)
func defaultHandler(cfg *Config, _ publish.Reporter) (request.Handler, error) {
	return withMiddleware(
		rootHandler(cfg.SecretToken),
		append(apmHandler(intakeResultIdToMonitoringInt))...), nil
}

func systemMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeSystemData(d, beaterConfig.AugmentEnabled)
}

func userMetaDataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeUserData(d, beaterConfig.AugmentEnabled)
}

func rumTransformConfig(beaterConfig *Config) (*transform.Config, error) {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		return nil, err
	}
	return &transform.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}, nil
}
