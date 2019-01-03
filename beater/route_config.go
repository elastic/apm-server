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
	"net/http"
	"regexp"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/processor/sourcemap"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

var (
	rootURL = "/"

	// intake v2
	BackendURL = "/intake/v2/events"
	RumURL     = "/intake/v2/rum/events"

	// assets
	SourcemapsURL = "/assets/v1/sourcemaps"
)

const burstMultiplier = 3

type routeType struct {
	wrappingHandler     func(*Config, http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) transform.Config
}

var IntakeRoutes = map[string]intakeRoute{
	BackendURL: backendRoute,
	RumURL:     rumRoute,
}

var AssetRoutes = map[string]assetRoute{
	SourcemapsURL: {sourcemapRouteType, sourcemap.Processor, sourcemapUploadDecoder},
}

var (
	backendRoute = intakeRoute{
		routeType{
			backendHandler,
			systemMetadataDecoder,
			func(*Config) transform.Config { return transform.Config{} },
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

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		requestTimeHandler(
			authHandler(beaterConfig.SecretToken, h)))
}

func rumHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
		requestTimeHandler(
			corsHandler(beaterConfig.RumConfig.AllowOrigins, h)))
}

func sourcemapHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func systemMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeSystemData(d, beaterConfig.AugmentEnabled)
}

func userMetaDataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeUserData(d, beaterConfig.AugmentEnabled)
}

func rumTransformConfig(beaterConfig *Config) transform.Config {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := transform.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}
	return config
}

type assetRoute struct {
	routeType
	processor.Processor
	topLevelRequestDecoder func(*Config) decoder.ReqDecoder
}

func (r *assetRoute) Handler(p processor.Processor, beaterConfig *Config, report publish.Reporter) http.Handler {
	handler := assetHandler{
		requestDecoder: r.configurableDecoder(beaterConfig, r.topLevelRequestDecoder(beaterConfig)),
		processor:      p,
		tconfig:        r.transformConfig(beaterConfig),
	}

	return r.wrappingHandler(beaterConfig, handler.Handle(beaterConfig, report))
}

type intakeRoute struct {
	routeType
}

func (r intakeRoute) Handler(url string, c *Config, report publish.Reporter) http.Handler {
	reqDecoder := r.configurableDecoder(
		c,
		func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil },
	)

	handler := intakeHandler{
		requestDecoder: reqDecoder,
		streamProcessor: &stream.StreamProcessor{
			Tconfig:      r.transformConfig(c),
			MaxEventSize: c.MaxEventSize,
		},
	}

	if url == RumURL {
		if rlc, err := NewRlCache(c.RumConfig.EventRate.LruSize, c.RumConfig.EventRate.Limit, burstMultiplier); err == nil {
			handler.rlc = rlc
		} else {
			logp.NewLogger("handler").Error(err.Error())
		}
	}

	return r.wrappingHandler(c, handler.Handle(c, report))
}
