package beater

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"

	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"net/http"

	"crypto/subtle"

	"github.com/hashicorp/golang-lru"
	"golang.org/x/time/rate"

	"github.com/ryanuber/go-glob"

	err "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/healthcheck"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"
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

type ProcessorFactory func(*http.Request) processor.Processor

type ProcessorHandler func(ProcessorFactory, Config, reporter) http.Handler

type routeMapping struct {
	ProcessorHandler
	ProcessorFactory
}

var (
	serverMetrics  = monitoring.Default.NewRegistry("apm-server.server")
	requestCounter = monitoring.NewInt(serverMetrics, "requests.counter")
	responseValid  = monitoring.NewInt(serverMetrics, "response.valid")
	responseErrors = monitoring.NewInt(serverMetrics, "response.errors")

	errInvalidToken    = errors.New("invalid token")
	errForbidden       = errors.New("forbidden request")
	errPOSTRequestOnly = errors.New("only POST requests are supported")
	errTooManyRequests = errors.New("too many requests")

	Routes = map[string]routeMapping{
		BackendTransactionsURL:  {backendHandler, transaction.NewProcessor},
		FrontendTransactionsURL: {frontendHandler, transaction.NewProcessor},
		BackendErrorsURL:        {backendHandler, err.NewProcessor},
		FrontendErrorsURL:       {frontendHandler, err.NewProcessor},
		HealthCheckURL:          {healthCheckHandler, healthcheck.NewProcessor},
		SourcemapsURL:           {sourcemapHandler, sourcemap.NewProcessor},
	}
)

func newMuxer(config Config, report reporter) *http.ServeMux {
	mux := http.NewServeMux()

	for path, mapping := range Routes {
		logp.Info("Path %s added to request handler", path)
		mux.Handle(path, mapping.ProcessorHandler(mapping.ProcessorFactory, config, report))
	}

	return mux
}

func backendHandler(pf ProcessorFactory, config Config, report reporter) http.Handler {
	return logHandler(
		authHandler(config.SecretToken,
			processRequestHandler(pf, config, report, decodeLimitJSONData(config.MaxUnzippedSize))))
}

func frontendHandler(pf ProcessorFactory, config Config, report reporter) http.Handler {
	return logHandler(
		killSwitchHandler(config.Frontend.isEnabled(),
			ipRateLimitHandler(config.Frontend.RateLimit,
				corsHandler(config.Frontend.AllowOrigins,
					processRequestHandler(pf, config, report, decodeLimitJSONData(config.MaxUnzippedSize))))))
}

func sourcemapHandler(pf ProcessorFactory, config Config, report reporter) http.Handler {
	return logHandler(
		killSwitchHandler(config.Frontend.isEnabled(),
			authHandler(config.SecretToken,
				processRequestHandler(pf, config, report, sourcemap.DecodeSourcemapFormData))))
}

func healthCheckHandler(_ ProcessorFactory, _ Config, _ reporter) http.Handler {
	return logHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, http.StatusOK, nil)
		}))
}

func logHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request: URI=%s, method=%s, content-length=%d", r.RequestURI, r.Method, r.ContentLength)
		requestCounter.Inc()
		h.ServeHTTP(w, r)
	})
}

func killSwitchHandler(killSwitch bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if killSwitch {
			h.ServeHTTP(w, r)
		} else {
			sendStatus(w, r, http.StatusForbidden, errForbidden)
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
		if deny(utility.ExtractIP(r)) {
			sendStatus(w, r, http.StatusTooManyRequests, errTooManyRequests)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			sendStatus(w, r, http.StatusUnauthorized, errInvalidToken)
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

			sendStatus(w, r, http.StatusOK, nil)

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			sendStatus(w, r, http.StatusForbidden, errForbidden)
		}
	})
}

func processRequestHandler(pf ProcessorFactory, config Config, report reporter, decode decoder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := processRequest(r, pf, config.MaxUnzippedSize, report, decode)
		sendStatus(w, r, code, err)
	})
}

func processRequest(r *http.Request, pf ProcessorFactory, maxSize int64, report reporter, decode decoder) (int, error) {

	processor := pf(r)

	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errPOSTRequestOnly
	}

	buf, err := decode(r)
	if err != nil {
		return http.StatusBadRequest, errors.New(fmt.Sprintf("Decoding error: %s", err.Error()))
	}

	if err = processor.Validate(buf); err != nil {
		return http.StatusBadRequest, err
	}

	list, err := processor.Transform(buf)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if err = report(list); err != nil {
		return http.StatusServiceUnavailable, err
	}

	return http.StatusAccepted, nil
}

type decoder func(req *http.Request) ([]byte, error)

func decodeLimitJSONData(maxSize int64) decoder {
	return func(req *http.Request) ([]byte, error) {

		contentType := req.Header.Get("Content-Type")
		if contentType != "application/json" {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		reader := req.Body
		if reader == nil {
			return nil, fmt.Errorf("No content supplied")
		}

		switch req.Header.Get("Content-Encoding") {
		case "deflate":
			var err error
			reader, err = zlib.NewReader(reader)
			if err != nil {
				return nil, err
			}

		case "gzip":
			var err error
			reader, err = gzip.NewReader(reader)
			if err != nil {
				return nil, err
			}
		}

		// Limit size of request to prevent for example zip bombs
		limitedReader := io.LimitReader(reader, maxSize)
		buf, err := ioutil.ReadAll(limitedReader)
		if err != nil {
			// If we run out of memory, for example
			return nil, errors.New(fmt.Sprintf("Data read error: %s", err.Error()))
		}

		return buf, nil
	}
}

func sendStatus(w http.ResponseWriter, r *http.Request, code int, err error) {
	content_type := "text/plain; charset=utf-8"
	if acceptsJSON(r) {
		content_type = "application/json"
	}
	w.Header().Set("Content-Type", content_type)
	w.WriteHeader(code)

	if err == nil {
		responseValid.Inc()
		logp.Debug("request", "request successful, code=%d", code)
		return
	}

	logp.Err("%s, code=%d", err.Error(), code)

	responseErrors.Inc()
	if acceptsJSON(r) {
		sendJSON(w, map[string]interface{}{"error": err.Error()})
	} else {
		sendPlain(w, err.Error())
	}
}

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendJSON(w http.ResponseWriter, msg map[string]interface{}) {
	buf, err := json.Marshal(msg)
	if err != nil {
		logp.Err("Error while generating a JSON error response: %v", err)
		return
	}

	w.Write(buf)
}

func sendPlain(w http.ResponseWriter, msg string) {
	w.Write([]byte(msg))
}
