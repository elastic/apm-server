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
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

var (
	rootURL                   = "/"
	BackendTransactionsURL    = "/v1/transactions"
	ClientSideTransactionsURL = "/v1/client-side/transactions"
	RumTransactionsURL        = "/v1/rum/transactions"
	BackendErrorsURL          = "/v1/errors"
	ClientSideErrorsURL       = "/v1/client-side/errors"
	RumErrorsURL              = "/v1/rum/errors"
	MetricsURL                = "/v1/metrics"
	SourcemapsClientSideURL   = "/v1/client-side/sourcemaps"
	SourcemapsURL             = "/v1/rum/sourcemaps"
	V2BackendURL              = "/v2/intake"
	V2RumURL                  = "/v2/rum/intake"

	HealthCheckURL = "/healthcheck"
)

type routeType struct {
	wrappingHandler     func(*Config, http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) transform.Config
}

var V1Routes = map[string]v1Route{
	BackendTransactionsURL:    {backendRouteType, transaction.Processor, v1RequestDecoder},
	ClientSideTransactionsURL: {rumRouteType, transaction.Processor, v1RequestDecoder},
	RumTransactionsURL:        {rumRouteType, transaction.Processor, v1RequestDecoder},
	BackendErrorsURL:          {backendRouteType, perr.Processor, v1RequestDecoder},
	ClientSideErrorsURL:       {rumRouteType, perr.Processor, v1RequestDecoder},
	RumErrorsURL:              {rumRouteType, perr.Processor, v1RequestDecoder},
	MetricsURL:                {metricsRouteType, metric.Processor, v1RequestDecoder},
	SourcemapsClientSideURL:   {sourcemapRouteType, sourcemap.Processor, sourcemapUploadDecoder},
	SourcemapsURL:             {sourcemapRouteType, sourcemap.Processor, sourcemapUploadDecoder},
}

var V2Routes = map[string]v2Route{
	V2BackendURL: v2BackendRoute,
	V2RumURL:     {rumRouteType},
}

var v2BackendRoute = v2Route{
	routeType{
		v2backendHandler,
		systemMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	},
}

var (
	backendRouteType = routeType{
		backendHandler,
		systemMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	rumRouteType = routeType{
		rumHandler,
		userMetaDataDecoder,
		rumTransformConfig,
	}
	metricsRouteType = routeType{
		metricsHandler,
		systemMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	sourcemapRouteType = routeType{
		sourcemapHandler,
		systemMetadataDecoder,
		rumTransformConfig,
	}

	v1RequestDecoder = func(beaterConfig *Config) decoder.ReqDecoder {
		return decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize)
	}

	sourcemapUploadDecoder = func(beaterConfig *Config) decoder.ReqDecoder {
		return decoder.DecodeSourcemapFormData
	}
)

func v2backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		requestTimeHandler(
			authHandler(beaterConfig.SecretToken, h)))
}

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		requestTimeHandler(
			concurrencyLimitHandler(beaterConfig,
				authHandler(beaterConfig.SecretToken, h))))
}

func rumHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
		requestTimeHandler(
			concurrencyLimitHandler(beaterConfig,
				ipRateLimitHandler(beaterConfig.RumConfig.RateLimit,
					corsHandler(beaterConfig.RumConfig.AllowOrigins, h)))))
}

func metricsHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		requestTimeHandler(
			killSwitchHandler(beaterConfig.Metrics.isEnabled(),
				authHandler(beaterConfig.SecretToken, h))))
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

type v1Route struct {
	routeType
	processor.Processor
	topLevelRequestDecoder func(*Config) decoder.ReqDecoder
}

func (v *v1Route) Handler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	decoder := v.configurableDecoder(beaterConfig, v.topLevelRequestDecoder(beaterConfig))
	tconfig := v.transformConfig(beaterConfig)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := processRequest(r, p, tconfig, report, decoder)
		sendStatus(w, r, res)
	})

	return v.wrappingHandler(beaterConfig, handler)
}

type v2Route struct {
	routeType
}

func (v v2Route) Handler(beaterConfig *Config, report reporter) http.Handler {
	reqDecoder := v.configurableDecoder(
		beaterConfig,
		func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil },
	)

	v2Handler := v2Handler{
		requestDecoder: reqDecoder,
		tconfig:        v.transformConfig(beaterConfig),
	}

	return v.wrappingHandler(beaterConfig, v2Handler.Handle(beaterConfig, report))
}
