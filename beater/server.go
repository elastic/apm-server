package beater

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
)

type successCallback func([]beat.Event)

func newServer(config Config, publish successCallback) *http.Server {
	mux := http.NewServeMux()

	for path, p := range processor.Registry.Processors() {

		handler := createHandler(p, config, publish)

		logp.Info("Path %s added to request handler", path)

		mux.HandleFunc(path, handler)
	}

	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	return &http.Server{
		Addr:           config.Host,
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
}

func run(server *http.Server, ssl *SSLConfig) error {
	logp.Info("starting apm-server! Hit CTRL-C to stop it.")
	if ssl.isEnabled() {
		return server.ListenAndServeTLS(ssl.Cert, ssl.PrivateKey)
	} else {
		return server.ListenAndServe()
	}
}

func stop(server *http.Server, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		logp.Err(err.Error())
		err = server.Close()
		if err != nil {
			logp.Err(err.Error())
		}
	}
}

type handler func(w http.ResponseWriter, r *http.Request)

func createHandler(p processor.Processor, config Config, publish successCallback) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request: URI=%s, method=%s, content-length=%d", r.RequestURI, r.Method, r.ContentLength)

		if !checkSecretToken(r, config.SecretToken) {
			sendError(w, r, 401, "Invalid token", false)
			return
		}

		if r.Method != "POST" {
			sendError(w, r, 405, "Only post requests are supported", false)
			return
		}

		reader, err := decodeData(r)
		if err != nil {
			sendError(w, r, 400, fmt.Sprintf("Decoding error: %s", err.Error()), false)
			return
		}
		defer reader.Close()

		// Limit size of request to prevent for example zip bombs
		limitedReader := io.LimitReader(reader, config.MaxUnzippedSize)

		buf, err := ioutil.ReadAll(limitedReader)
		if err != nil {
			// If we run out of memory, for example
			sendError(w, r, 500, fmt.Sprintf("Data read error: %s", err), true)
		}

		err = p.Validate(buf)
		if err != nil {
			sendError(w, r, 400, fmt.Sprintf("Data validation error: %s", err), false)
			return
		}

		list, err := p.Transform(buf)
		if err != nil {
			sendError(w, r, 500, fmt.Sprintf("Data transformation error: %s", err), true)
			return
		}

		w.WriteHeader(202)
		publish(list)
	}
}

func sendError(w http.ResponseWriter, r *http.Request, code int, error string, log bool) {
	if log {
		logp.Err(error)
	} else {
		logp.Info("%s, code=%d", error, code)
	}

	w.WriteHeader(code)
	acceptHeader := r.Header.Get("Accept")
	// send JSON if the client will accept it
	if strings.Contains(acceptHeader, "*/*") || strings.Contains(acceptHeader, "application/json") {
		buf, err := json.Marshal(map[string]interface{}{
			"error": error,
		})

		if err != nil {
			logp.Err("Error while generating a JSON error response: %v", err)
			return
		}

		w.Write(buf)
	} else {
		w.Write([]byte(error))
	}
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
