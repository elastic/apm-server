package beater

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"flag"
	"io/ioutil"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
)

var debugHandler = flag.Bool("debugHandler", false, "Enable for having catchall endpoint Enabled")

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

	// TODO: Remove or find nicer way before going GA
	if *debugHandler {
		logp.Warn("Debug handler Enabled")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			reader, err := decodeData(r)
			if err != nil {
				logp.Err("Decode error: %s", err)
			}

			body, _ := ioutil.ReadAll(reader)
			logp.Info("Request Body: %+v", string(body))
		})
	}

	return &http.Server{
		Addr:           config.Host,
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
}

func start(server *http.Server, ssl *SSLConfig) {
	if ssl.isEnabled() {
		go server.ListenAndServeTLS(ssl.Cert, ssl.PrivateKey)
	} else {
		go server.ListenAndServe()
	}
}

func stop(server *http.Server) error {
	c := context.TODO()
	err := server.Shutdown(c)
	return err
}

type handler func(w http.ResponseWriter, r *http.Request)

func createHandler(p processor.Processor, config Config, publish successCallback) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request: URI=%s, method=%s, content-length=%d", r.RequestURI, r.Method, r.ContentLength)

		if !checkSecretToken(r, config.SecretToken) {
			sendError(w, r, 401, "Invalid token", true)
			return
		}

		if r.Method != "POST" {
			sendError(w, r, 405, "Only post requests are supported", false)
			return
		}

		reader, err := decodeData(r)
		if err != nil {
			sendError(w, r, 400, fmt.Sprintf("Decoding error: %s", err.Error()), true)
			return
		}
		defer reader.Close()

		// Limit size of request to prevent for example zip bombs
		limitedReader := io.LimitReader(reader, config.MaxUnzippedSize)
		err = p.Validate(limitedReader)
		if err != nil {
			sendError(w, r, 400, fmt.Sprintf("Data Validation error: %s", err), true)
			return
		}

		list := p.Transform()

		w.WriteHeader(202)
		publish(list)
	}
}

func sendError(w http.ResponseWriter, r *http.Request, code int, error string, log bool) {
	if log {
		logp.Err(error)
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
