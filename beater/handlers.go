package beater

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/logp"

	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"net/http"

	"crypto/subtle"

	err "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/healthcheck"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	BackendTransactionsURL  = "/v1/transactions"
	FrontendTransactionsURL = "/v1/client-side/transactions"
	BackendErrorsURL        = "/v1/errors"
	FrontendErrorsURL       = "/v1/client-side/errors"
	HealthCheckURL          = "/healthcheck"

	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type ProcessorFactory func() processor.Processor

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

	Routes = map[string]routeMapping{
		BackendTransactionsURL:  {backendHandler, transaction.NewProcessor},
		FrontendTransactionsURL: {frontendHandler, transaction.NewProcessor},
		BackendErrorsURL:        {backendHandler, err.NewProcessor},
		FrontendErrorsURL:       {frontendHandler, err.NewProcessor},
		HealthCheckURL:          {healthCheckHandler, healthcheck.NewProcessor},
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
			processRequestHandler(pf, config, report)))
}

func frontendHandler(pf ProcessorFactory, config Config, report reporter) http.Handler {
	return logHandler(
		frontendSwitchHandler(config.EnableFrontend,
			corsHandler(config.AllowOrigins,
				processRequestHandler(pf, config, report))))
}

func healthCheckHandler(_ ProcessorFactory, _ Config, _ reporter) http.Handler {
	return logHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, 200, nil)
		}))
}

func logHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request: URI=%s, method=%s, content-length=%d", r.RequestURI, r.Method, r.ContentLength)
		requestCounter.Inc()
		h.ServeHTTP(w, r)
	})
}

func frontendSwitchHandler(feSwitch bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if feSwitch {
			h.ServeHTTP(w, r)
		} else {
			sendStatus(w, r, 403, errForbidden)
		}
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			sendStatus(w, r, 401, errInvalidToken)
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
			if origin == allowed || allowed == "*" {
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

			sendStatus(w, r, 200, nil)

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			sendStatus(w, r, 403, errForbidden)
		}
	})
}

func processRequestHandler(pf ProcessorFactory, config Config, report reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := processRequest(r, pf, config.MaxUnzippedSize, report)
		sendStatus(w, r, code, err)
	})
}

func processRequest(r *http.Request, pf ProcessorFactory, maxSize int64, report reporter) (int, error) {

	processor := pf()

	if r.Method != "POST" {
		return 405, errPOSTRequestOnly
	}

	reader, err := decodeData(r)
	if err != nil {
		return 400, errors.New(fmt.Sprintf("Decoding error: %s", err.Error()))
	}
	defer reader.Close()

	// Limit size of request to prevent for example zip bombs
	limitedReader := io.LimitReader(reader, maxSize)
	buf, err := ioutil.ReadAll(limitedReader)
	if err != nil {
		// If we run out of memory, for example
		return 500, errors.New(fmt.Sprintf("Data read error: %s", err.Error()))

	}

	if err = processor.Validate(buf); err != nil {
		return 400, err
	}

	list, err := processor.Transform(buf)
	if err != nil {
		return 400, err
	}

	if err = report(list); err != nil {
		return 503, err
	}

	return 202, nil
}

func decodeData(req *http.Request) (io.ReadCloser, error) {

	if req.Header.Get("Content-Type") != "application/json" {
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

	return reader, nil
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
