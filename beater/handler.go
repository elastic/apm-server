package beater

import (
	"compress/gzip"
	"compress/zlib"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/logp"
)

var (
	errInvalidToken    = errors.New("Invalid token")
	errPOSTRequestOnly = errors.New("Only post requests are supported")
)

type handler func(w http.ResponseWriter, r *http.Request)

func createHandler(p processor.Processor, config Config, report reporter) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request: URI=%s, method=%s, content-length=%d", r.RequestURI, r.Method, r.ContentLength)
		requestCounter.Inc()

		code, err := processRequest(r, p, config, report)
		sendStatus(w, r, code, err)
	}
}

func processRequest(r *http.Request, p processor.Processor, config Config, report reporter) (code int, err error) {
	if !checkSecretToken(r, config.SecretToken) {
		return reportInfo(401, errInvalidToken)
	}
	if r.Method != "POST" {
		return reportInfo(405, errPOSTRequestOnly)
	}

	reader, err := decodeData(r)
	if err != nil {
		return reportInfo(400, fmt.Errorf("Decoding error: %s", err.Error()))
	}
	defer reader.Close()

	// Limit size of request to prevent for example zip bombs
	limitedReader := io.LimitReader(reader, config.MaxUnzippedSize)
	buf, err := ioutil.ReadAll(limitedReader)
	if err != nil {
		// If we run out of memory, for example
		return reportError(500, fmt.Errorf("Data read error: %s", err))
	}

	if err = p.Validate(buf); err != nil {
		return reportInfo(400, fmt.Errorf("Data validation error: %s", err))
	}

	list, err := p.Transform(buf)
	if err != nil {
		return reportError(500, fmt.Errorf("Data transformation error: %s", err))
	}

	responseValid.Inc()

	if err = report(list); err != nil {
		return reportError(503, fmt.Errorf("Error adding data to internal queue: %s", err))
	}
	return 202, nil
}

func reportError(code int, err error) (int, error) {
	logp.Err(err.Error())
	return code, err
}

func reportInfo(code int, err error) (int, error) {
	logp.Info("%s, code=%d", err.Error(), code)
	return code, err
}

// checkSecretToken checks the Authorization header. It must be in the form of:
//
//   Authorization: Bearer <secret-token>
//
// Bearer must be part of it.
func checkSecretToken(req *http.Request, secretToken string) bool {
	// No token configured
	if secretToken == "" {
		return true
	}
	header := req.Header.Get("Authorization")

	parts := strings.Split(header, " ")

	if len(parts) != 2 {
		// No access
		return false
	}

	if parts[0] != "Bearer" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secretToken)) == 1
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

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendStatus(w http.ResponseWriter, r *http.Request, code int, err error) {
	w.WriteHeader(code)
	if err != nil {
		responseErrors.Inc()
		if acceptsJSON(r) {
			w.Header().Add("Content-Type", "application/json")
			sendJSON(w, map[string]interface{}{"error": err.Error()})
		} else {
			w.Header().Add("Content-Type", "text/plain; charset=UTF-8")
			sendPlain(w, err.Error())
		}
	}
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
