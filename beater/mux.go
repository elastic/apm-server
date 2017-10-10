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

	"github.com/elastic/beats/libbeat/monitoring"
)

type chainable func(processor.Processor, Config, reporter) http.Handler

var (
	serverMetrics  = monitoring.Default.NewRegistry("apm-server.server")
	requestCounter = monitoring.NewInt(serverMetrics, "requests.counter")
	responseValid  = monitoring.NewInt(serverMetrics, "response.valid")
	responseErrors = monitoring.NewInt(serverMetrics, "response.errors")

	errInvalidToken    = errors.New("invalid token")
	errPOSTRequestOnly = errors.New("only POST requests are supported")

	handlerMap = map[int]chainable{
		processor.Backend:  backendHandler,
		processor.Frontend: frontendHandler,
		processor.Nop:      nopHandler,
	}
)

func newMuxer(config Config, report reporter) *http.ServeMux {
	mux := http.NewServeMux()

	for path, p := range processor.Registry.Processors() {
		handler := handlerMap[p.Type()]
		logp.Info("Path %s added to request handler", path)
		mux.Handle(path, handler(p, config, report))
	}

	return mux
}

func backendHandler(p processor.Processor, config Config, report reporter) http.Handler {
	return logHandler(
		authHandler(config.SecretToken,
			appHandler(p, config, report)))
}

func frontendHandler(p processor.Processor, config Config, report reporter) http.Handler {
	return logHandler(
		appHandler(p, config, report))
}

func nopHandler(_ processor.Processor, _ Config, _ reporter) http.Handler {
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

func appHandler(p processor.Processor, config Config, report reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := processRequest(r, p, config.MaxUnzippedSize, report)
		sendStatus(w, r, code, err)
	})
}

func processRequest(r *http.Request, p processor.Processor, maxSize int64, report reporter) (int, error) {

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

	if err = p.Validate(buf); err != nil {
		return 400, err
	}

	list, err := p.Transform(buf)
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
