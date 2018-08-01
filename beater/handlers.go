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
	"context"
	"crypto/subtle"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"
	"github.com/satori/go.uuid"
	"golang.org/x/time/rate"

	conf "github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor"
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/elastic/beats/libbeat/version"
)

const (
	rootURL                   = "/"
	BackendTransactionsURL    = "/v1/transactions"
	ClientSideTransactionsURL = "/v1/client-side/transactions"
	RumTransactionsURL        = "/v1/rum/transactions"
	BackendErrorsURL          = "/v1/errors"
	ClientSideErrorsURL       = "/v1/client-side/errors"
	RumErrorsURL              = "/v1/rum/errors"
	HealthCheckURL            = "/healthcheck"
	MetricsURL                = "/v1/metrics"
	SourcemapsClientSideURL   = "/v1/client-side/sourcemaps"
	SourcemapsURL             = "/v1/rum/sourcemaps"

	rateLimitCacheSize       = 1000
	rateLimitBurstMultiplier = 2

	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type ProcessorHandler func(processor.Processor, *Config, reporter) http.Handler

type routeMapping struct {
	ProcessorHandler
	processor.Processor
}

type serverResponse struct {
	err     error
	code    int
	counter *monitoring.Int
	body    interface{}
}

var (
	serverMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counter       = func(s string) *monitoring.Int {
		return monitoring.NewInt(serverMetrics, s)
	}
	requestCounter    = counter("request.count")
	concurrentWait    = counter("concurrent.wait.ms")
	responseCounter   = counter("response.count")
	responseErrors    = counter("response.errors.count")
	responseSuccesses = counter("response.valid.count")
	responseOk        = counter("response.valid.ok")

	okResponse = serverResponse{
		code:    http.StatusOK,
		counter: responseOk,
	}
	acceptedResponse = serverResponse{
		code:    http.StatusAccepted,
		counter: counter("response.valid.accepted"),
	}
	forbiddenCounter  = counter("response.errors.forbidden")
	forbiddenResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "forbidden request"),
			code:    http.StatusForbidden,
			counter: forbiddenCounter,
		}
	}
	unauthorizedResponse = serverResponse{
		err:     errors.New("invalid token"),
		code:    http.StatusUnauthorized,
		counter: counter("response.errors.unauthorized"),
	}
	requestTooLargeResponse = serverResponse{
		err:     errors.New("request body too large"),
		code:    http.StatusRequestEntityTooLarge,
		counter: counter("response.errors.toolarge"),
	}
	decodeCounter        = counter("response.errors.decode")
	cannotDecodeResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "data decoding error"),
			code:    http.StatusBadRequest,
			counter: decodeCounter,
		}
	}
	validateCounter        = counter("response.errors.validate")
	cannotValidateResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "data validation error"),
			code:    http.StatusBadRequest,
			counter: validateCounter,
		}
	}
	rateLimitedResponse = serverResponse{
		err:     errors.New("too many requests"),
		code:    http.StatusTooManyRequests,
		counter: counter("response.errors.ratelimit"),
	}
	methodNotAllowedResponse = serverResponse{
		err:     errors.New("only POST requests are supported"),
		code:    http.StatusMethodNotAllowed,
		counter: counter("response.errors.method"),
	}
	tooManyConcurrentRequestsResponse = serverResponse{
		err:     errors.New("timeout waiting to be processed"),
		code:    http.StatusServiceUnavailable,
		counter: counter("response.errors.concurrency"),
	}
	fullQueueCounter  = counter("response.errors.queue")
	fullQueueResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.New("queue is full"),
			code:    http.StatusServiceUnavailable,
			counter: fullQueueCounter,
		}
	}
	serverShuttingDownCounter  = counter("response.errors.closed")
	serverShuttingDownResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.New("server is shutting down"),
			code:    http.StatusServiceUnavailable,
			counter: serverShuttingDownCounter,
		}
	}

	ProcessorRoutes = map[string]routeMapping{
		BackendTransactionsURL:    {backendHandler, transaction.Processor},
		ClientSideTransactionsURL: {rumHandler, transaction.Processor},
		RumTransactionsURL:        {rumHandler, transaction.Processor},
		BackendErrorsURL:          {backendHandler, perr.Processor},
		ClientSideErrorsURL:       {rumHandler, perr.Processor},
		RumErrorsURL:              {rumHandler, perr.Processor},
		MetricsURL:                {metricsHandler, metric.Processor},
		SourcemapsClientSideURL:   {sourcemapHandler, sourcemap.Processor},
		SourcemapsURL:             {sourcemapHandler, sourcemap.Processor},
	}
)

func newMuxer(beaterConfig *Config, report reporter) *http.ServeMux {
	mux := http.NewServeMux()
	logger := logp.NewLogger("handler")
	for path, mapping := range ProcessorRoutes {
		logger.Infof("Path %s added to request handler", path)

		mux.Handle(path, mapping.ProcessorHandler(mapping.Processor, beaterConfig, report))
	}

	mux.Handle(rootURL, rootHandler(beaterConfig.SecretToken))
	mux.Handle(HealthCheckURL, healthCheckHandler())

	if beaterConfig.Expvar.isEnabled() {
		path := beaterConfig.Expvar.Url
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux
}

func concurrencyLimitHandler(beaterConfig *Config, h http.Handler) http.Handler {
	semaphore := make(chan struct{}, beaterConfig.ConcurrentRequests)
	release := func() {
		<-semaphore
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		var wait = func() int64 {
			return time.Now().Sub(t).Nanoseconds() / 1e6
		}
		select {
		case semaphore <- struct{}{}:
			concurrentWait.Add(wait())
			defer release()
			h.ServeHTTP(w, r)
		case <-time.After(beaterConfig.MaxRequestQueueTime):
			concurrentWait.Add(wait())
			sendStatus(w, r, tooManyConcurrentRequestsResponse)
		}

	})
}

func backendHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, conf.Config{}, report,
					decoder.DecodeSystemData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))
}

func rumHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := conf.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			concurrencyLimitHandler(beaterConfig,
				ipRateLimitHandler(beaterConfig.RumConfig.RateLimit,
					corsHandler(beaterConfig.RumConfig.AllowOrigins,
						processRequestHandler(p, config, report,
							decoder.DecodeUserData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))))
}

func metricsHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Metrics.isEnabled(),
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, conf.Config{}, report,
					decoder.DecodeSystemData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))
}

func sourcemapHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, conf.Config{SmapMapper: smapper}, report, decoder.DecodeSourcemapFormData))))
}

// Deprecated: use rootHandler instead
func healthCheckHandler() http.Handler {
	return logHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, okResponse)
		}))
}

func rootHandler(secretToken string) http.Handler {
	serverInfo := common.MapStr{
		"build_date": version.BuildTime().Format(time.RFC3339),
		"build_sha":  version.Commit(),
		"version":    version.GetDefaultVersion(),
	}
	detailedOkResponse := serverResponse{
		code:    http.StatusOK,
		counter: responseOk,
		body:    serverInfo,
	}

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if isAuthorized(r, secretToken) {
			sendStatus(w, r, detailedOkResponse)
			return
		}
		sendStatus(w, r, okResponse)
	})
	return logHandler(handler)
}

type logContextKey string

var reqLoggerContextKey = logContextKey("requestLogger")

func logHandler(h http.Handler) http.Handler {
	logger := logp.NewLogger("request")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := uuid.NewV4()

		requestCounter.Inc()

		reqLogger := logger.With(
			"request_id", reqID,
			"method", r.Method,
			"URL", r.URL,
			"content_length", r.ContentLength,
			"remote_address", utility.RemoteAddr(r),
			"user-agent", r.Header.Get("User-Agent"))

		lr := r.WithContext(
			context.WithValue(r.Context(), reqLoggerContextKey, reqLogger),
		)

		lw := utility.NewRecordingResponseWriter(w)

		h.ServeHTTP(lw, lr)

		if lw.Code <= 399 {
			reqLogger.Infow("handled request", []interface{}{"response_code", lw.Code}...)
		}
	})
}

func killSwitchHandler(killSwitch bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if killSwitch {
			h.ServeHTTP(w, r)
		} else {
			sendStatus(w, r, forbiddenResponse(errors.New("endpoint is disabled")))
		}
	})
}

func ipRateLimitHandler(rateLimit int, h http.Handler) http.Handler {
	cache, _ := lru.New(rateLimitCacheSize)

	var deny = func(ip string) bool {
		if !cache.Contains(ip) {
			cache.Add(ip, rate.NewLimiter(rate.Limit(rateLimit), rateLimit*rateLimitBurstMultiplier))
		}
		var limiter, _ = cache.Get(ip)
		return !limiter.(*rate.Limiter).Allow()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deny(utility.RemoteAddr(r)) {
			sendStatus(w, r, rateLimitedResponse)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			sendStatus(w, r, unauthorizedResponse)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// isAuthorized checks the Authorization header. It must be in the form of:
//   Authorization: Bearer <secret-token>
// Bearer must be part of it.
func isAuthorized(req *http.Request, secretToken string) bool {
	// No token configured
	if secretToken == "" {
		return true
	}
	header := req.Header.Get("Authorization")
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secretToken)) == 1
}

func corsHandler(allowedOrigins []string, h http.Handler) http.Handler {

	var isAllowed = func(origin string) bool {
		for _, allowed := range allowedOrigins {
			if glob.Glob(allowed, origin) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// origin header is always set by the browser
		origin := r.Header.Get("Origin")
		validOrigin := isAllowed(origin)

		if r.Method == "OPTIONS" {

			// setting the ACAO header is the way to tell the browser to go ahead with the request
			if validOrigin {
				// do not set the configured origin(s), echo the received origin instead
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			// tell browsers to cache response requestHeaders for up to 1 hour (browsers might ignore this)
			w.Header().Set("Access-Control-Max-Age", "3600")
			// origin must be part of the cache key so that we can handle multiple allowed origins
			w.Header().Set("Vary", "Origin")

			// required if Access-Control-Request-Method and Access-Control-Request-Headers are in the requestHeaders
			w.Header().Set("Access-Control-Allow-Methods", supportedMethods)
			w.Header().Set("Access-Control-Allow-Headers", supportedHeaders)

			w.Header().Set("Content-Length", "0")

			sendStatus(w, r, okResponse)

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			sendStatus(w, r, forbiddenResponse(errors.New("origin: '"+origin+"' is not allowed")))
		}
	})
}

func processRequestHandler(p processor.Processor, config conf.Config, report reporter, decode decoder.Decoder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := processRequest(r, p, config, report, decode)
		sendStatus(w, r, res)
	})
}

func processRequest(r *http.Request, p processor.Processor, config conf.Config, report reporter, decode decoder.Decoder) serverResponse {
	if r.Method != "POST" {
		return methodNotAllowedResponse
	}

	data, err := decode(r)
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return requestTooLargeResponse
		}
		return cannotDecodeResponse(err)

	}

	if err = p.Validate(data); err != nil {
		return cannotValidateResponse(err)
	}

	payload, err := p.Decode(data)
	if err != nil {
		return cannotDecodeResponse(err)
	}

	if err = report(r.Context(), pendingReq{payload: payload, config: config}); err != nil {
		if strings.Contains(err.Error(), "publisher is being stopped") {
			return serverShuttingDownResponse(err)
		}
		return fullQueueResponse(err)
	}

	return acceptedResponse
}

func sendStatus(w http.ResponseWriter, r *http.Request, res serverResponse) {
	contentType := "text/plain; charset=utf-8"
	if acceptsJSON(r) {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(res.code)

	responseCounter.Inc()
	res.counter.Inc()

	var msgKey string
	var msg interface{}
	if res.err == nil {
		responseSuccesses.Inc()
		if res.body == nil {
			return
		}
		msgKey = "ok"
		msg = res.body
	} else {
		responseErrors.Inc()

		logger, ok := r.Context().Value(reqLoggerContextKey).(*logp.Logger)
		if !ok {
			logger = logp.NewLogger("request")
		}
		msgKey = "error"
		msg = res.err.Error()
		logger.Errorw("error handling request", "response_code", res.code, "error", msg)
	}

	if acceptsJSON(r) {
		sendJSON(w, map[string]interface{}{msgKey: msg})
	} else {
		sendPlain(w, fmt.Sprintf("%s", msg))
	}
}

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendJSON(w http.ResponseWriter, msg map[string]interface{}) {
	buf, err := json.Marshal(msg)
	if err != nil {
		logp.NewLogger("response").Errorf("Error while generating a JSON error response: %v", err)
		return
	}

	w.Write(buf)
}

func sendPlain(w http.ResponseWriter, msg string) {
	w.Write([]byte(msg))
}
