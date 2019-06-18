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

	"github.com/elastic/beats/libbeat/kibana"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/asset"
	"github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

const (
	rootURL = "/"

	agentConfigURL = "/v1/agent/configs"

	// intake v2
	backendURL = "/intake/v2/events"
	rumURL     = "/intake/v2/rum/events"

	// assets
	sourcemapsURL = "/assets/v1/sourcemaps"

	burstMultiplier = 3
)

type routeType struct {
	wrappingHandler     func(*Config, http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) (*transform.Config, error)
}

var IntakeRoutes = map[string]intakeRoute{
	backendURL: backendRoute,
	rumURL:     rumRoute,
}

var AssetRoutes = map[string]assetRoute{
	sourcemapsURL: {sourcemapRouteType, sourcemap.Processor, sourcemapUploadDecoder},
}

var (
	backendRoute = intakeRoute{
		routeType{
			backendHandler,
			systemMetadataDecoder,
			func(*Config) (*transform.Config, error) { return &transform.Config{}, nil },
		},
	}
	rumRoute = intakeRoute{
		routeType{
			rumHandler,
			userMetaDataDecoder,
			rumTransformConfig,
		},
	}

	sourcemapRouteType = routeType{
		sourcemapHandler,
		systemMetadataDecoder,
		rumTransformConfig,
	}

	sourcemapUploadDecoder = func(beaterConfig *Config) decoder.ReqDecoder {
		return decoder.DecodeSourcemapFormData
	}
)

func newMuxer(beaterConfig *Config, kbClient *kibana.Client, report publish.Reporter) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	logger := logp.NewLogger("handler")

	for path, route := range AssetRoutes {
		logger.Infof("Path %s added to request handler", path)
		handler, err := route.Handler(route.Processor, beaterConfig, report)
		if err != nil {
			return nil, err
		}
		mux.Handle(path, handler)
	}
	for path, route := range IntakeRoutes {
		logger.Infof("Path %s added to request handler", path)

		handler, err := route.Handler(path, beaterConfig, report)
		if err != nil {
			return nil, err
		}
		mux.Handle(path, handler)
	}

	mux.Handle(agentConfigURL, requestCountHandler(agentConfigHandler(kbClient, beaterConfig.SecretToken)))
	mux.Handle(rootURL, requestCountHandler(rootHandler(beaterConfig.SecretToken)))

	if beaterConfig.Expvar.isEnabled() {
		path := beaterConfig.Expvar.Url
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux, nil
}

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return requestCountHandler(
		logHandler(
			requestTimeHandler(
				authHandler(beaterConfig.SecretToken, h))))
}

func rumHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return requestCountHandler(
		logHandler(
			killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
				requestTimeHandler(
					corsHandler(beaterConfig.RumConfig.AllowOrigins, h)))))
}

func sourcemapHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return requestCountHandler(
		logHandler(
			killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
				authHandler(beaterConfig.SecretToken, h))))
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

type assetRoute struct {
	routeType
	asset.Processor
	topLevelRequestDecoder func(*Config) decoder.ReqDecoder
}

func (r *assetRoute) Handler(p asset.Processor, beaterConfig *Config, report publish.Reporter) (http.Handler, error) {
	config, err := r.transformConfig(beaterConfig)
	if err != nil {
		return nil, err
	}
	handler := assetHandler{
		requestDecoder: r.configurableDecoder(beaterConfig, r.topLevelRequestDecoder(beaterConfig)),
		processor:      p,
		tconfig:        *config,
	}

	return r.wrappingHandler(beaterConfig, handler.Handle(beaterConfig, report)), nil
}

type intakeRoute struct {
	routeType
}

func (r intakeRoute) Handler(url string, c *Config, report publish.Reporter) (http.Handler, error) {
	reqDecoder := r.configurableDecoder(
		c,
		func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil },
	)
	config, err := r.transformConfig(c)
	if err != nil {
		return nil, err
	}
	handler := intakeHandler{
		requestDecoder: reqDecoder,
		streamProcessor: &stream.Processor{
			Tconfig:      *config,
			Mconfig:      model.Config{Experimental: c.Mode == ModeExperimental},
			MaxEventSize: c.MaxEventSize,
		},
	}

	if url == rumURL {
		if c, err := newRlCache(c.RumConfig.EventRate.LruSize, c.RumConfig.EventRate.Limit, burstMultiplier); err == nil {
			handler.rlc = c
		} else {
			return nil, err
		}
	}

	return r.wrappingHandler(c, handler.Handle(c, report)), nil
}
