package beater

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"expvar"
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
	"github.com/elastic/apm-server/processor/healthcheck"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	BackendTransactionsURL  = "/v1/transactions"
	FrontendTransactionsURL = "/v1/client-side/transactions"
	BackendErrorsURL        = "/v1/errors"
	FrontendErrorsURL       = "/v1/client-side/errors"
	HealthCheckURL          = "/healthcheck"
	SourcemapsURL           = "/v1/client-side/sourcemaps"

	rateLimitCacheSize       = 1000
	rateLimitBurstMultiplier = 2

	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type ProcessorFactory func() processor.Processor

type ProcessorHandler func(ProcessorFactory, *Config, reporter) http.Handler

type routeMapping struct {
	ProcessorHandler
	ProcessorFactory
}

type serverResponse struct {
	err     error
	code    int
	counter *monitoring.Int
}

var (
	serverMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counter       = func(s string) *monitoring.Int {
		return monitoring.NewInt(serverMetrics, s)
	}
	requestCounter    = counter("request.count")
	responseCounter   = counter("response.count")
	responseErrors    = counter("response.errors.count")
	responseSuccesses = counter("response.valid.count")

	okResponse = serverResponse{
		nil, http.StatusOK, counter("response.valid.ok"),
	}
	acceptedResponse = serverResponse{
		nil, http.StatusAccepted, counter("response.valid.accepted"),
	}
	forbiddenCounter  = counter("response.errors.forbidden")
	forbiddenResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "forbidden request"), http.StatusForbidden, forbiddenCounter,
		}
	}
	unauthorizedResponse = serverResponse{
		errors.New("invalid token"), http.StatusUnauthorized, counter("response.errors.unauthorized"),
	}
	requestTooLargeResponse = serverResponse{
		errors.New("request body too large"), http.StatusRequestEntityTooLarge, counter("response.errors.toolarge"),
	}
	decodeCounter        = counter("response.errors.decode")
	cannotDecodeResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "data decoding error"), http.StatusBadRequest, decodeCounter,
		}
	}
	validateCounter        = counter("response.errors.validate")
	cannotValidateResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "data validation error"), http.StatusBadRequest, validateCounter,
		}
	}
	rateLimitedResponse = serverResponse{
		errors.New("too many requests"), http.StatusTooManyRequests, counter("response.errors.ratelimit"),
	}
	methodNotAllowedResponse = serverResponse{
		errors.New("only POST requests are supported"), http.StatusMethodNotAllowed, counter("response.errors.method"),
	}
	tooManyConcurrentRequestsResponse = serverResponse{
		errors.New("timeout waiting to be processed"), http.StatusServiceUnavailable, counter("response.errors.concurrency"),
	}
	fullQueueCounter  = counter("response.errors.queue")
	fullQueueResponse = func(err error) serverResponse {
		return serverResponse{
			errors.New("queue is full"), http.StatusServiceUnavailable, fullQueueCounter,
		}
	}
	serverShuttingDownCounter  = counter("response.errors.closed")
	serverShuttingDownResponse = func(err error) serverResponse {
		return serverResponse{
			errors.New("server is shutting down"), http.StatusServiceUnavailable, serverShuttingDownCounter,
		}
	}

	Routes = map[string]routeMapping{
		BackendTransactionsURL:  {backendHandler, transaction.NewProcessor},
		FrontendTransactionsURL: {frontendHandler, transaction.NewProcessor},
		BackendErrorsURL:        {backendHandler, perr.NewProcessor},
		FrontendErrorsURL:       {frontendHandler, perr.NewProcessor},
		HealthCheckURL:          {healthCheckHandler, healthcheck.NewProcessor},
		SourcemapsURL:           {sourcemapHandler, sourcemap.NewProcessor},
	}
)

func newMuxer(beaterConfig *Config, report reporter) *http.ServeMux {
	mux := http.NewServeMux()
	logger := logp.NewLogger("handler")
	for path, mapping := range Routes {
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, mapping.ProcessorHandler(mapping.ProcessorFactory, beaterConfig, report))
	}

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
		select {
		case semaphore <- struct{}{}:
			defer release()
			h.ServeHTTP(w, r)
		case <-time.After(beaterConfig.MaxRequestQueueTime):
			sendStatus(w, r, tooManyConcurrentRequestsResponse)
		}

	})
}

func backendHandler(pf ProcessorFactory, beaterConfig *Config, report reporter) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(pf, conf.Config{}, report,
					decoder.DecodeSystemData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))
}

func frontendHandler(pf ProcessorFactory, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.Frontend.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := conf.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.Frontend.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.Frontend.ExcludeFromGrouping),
	}
	return logHandler(
		killSwitchHandler(beaterConfig.Frontend.isEnabled(),
			concurrencyLimitHandler(beaterConfig,
				ipRateLimitHandler(beaterConfig.Frontend.RateLimit,
					corsHandler(beaterConfig.Frontend.AllowOrigins,
						processRequestHandler(pf, config, report,
							decoder.DecodeUserData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))))
}

func sourcemapHandler(pf ProcessorFactory, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.Frontend.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	return logHandler(
		killSwitchHandler(beaterConfig.Frontend.isEnabled(),
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(pf, conf.Config{SmapMapper: smapper}, report, decoder.DecodeSourcemapFormData))))
}

func healthCheckHandler(_ ProcessorFactory, _ *Config, _ reporter) http.Handler {
	return logHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, okResponse)
		}))
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

func processRequestHandler(pf ProcessorFactory, config conf.Config, report reporter, decode decoder.Decoder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := processRequest(r, pf, config, report, decode)
		sendStatus(w, r, res)
	})
}

func processRequest(r *http.Request, pf ProcessorFactory, config conf.Config, report reporter, decode decoder.Decoder) serverResponse {
	processor := pf()

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

	if err = processor.Validate(data); err != nil {
		return cannotValidateResponse(err)
	}

	payload, err := processor.Decode(data)
	if err != nil {
		return cannotDecodeResponse(err)
	}

	if err = report(pendingReq{payload: payload, config: config}); err != nil {
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
	if res.err == nil {
		responseSuccesses.Inc()
		return
	}

	logger, ok := r.Context().Value(reqLoggerContextKey).(*logp.Logger)
	if !ok {
		logger = logp.NewLogger("request")
	}
	errMsg := res.err.Error()
	logger.Errorw("error handling request", "response_code", res.code, "error", errMsg)

	responseErrors.Inc()

	if acceptsJSON(r) {
		sendJSON(w, map[string]interface{}{"error": errMsg})
	} else {
		sendPlain(w, errMsg)
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
