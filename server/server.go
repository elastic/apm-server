package server

import (
	"compress/zlib"
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"compress/gzip"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher/beat"
)

var debugHandler = flag.Bool("debugHandler", false, "Enable for having catchall endpoint enabled")

type Server struct {
	config Config
	http   *http.Server
}

func New(cfg *common.Config) (*Server, error) {
	config := defaultConfig
	if cfg != nil {
		if err := cfg.Unpack(&config); err != nil {
			return nil, fmt.Errorf("Error unpacking config: %v", err)
		}
	}

	return &Server{config: config}, nil
}

func (s *Server) create(successCallback func([]beat.Event), host string) *http.Server {
	mux := http.NewServeMux()

	for path, p := range processor.Registry.GetProcessors() {

		handler := s.createHandler(p, successCallback)

		logp.Info("Path %s added to request handler", path)

		mux.HandleFunc(path, handler)
	}

	// healthcheck handler, which always returns a 200 response, indicating
	// that we're ready to work

	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// TODO: Remove or find nicer way before going GA
	if *debugHandler {
		logp.Warn("Debug handler enabled")
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
		Addr:           host,
		Handler:        mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}
}

func (s *Server) Start(successCallback func([]beat.Event), host string) {
	s.http = s.create(successCallback, host)
	go s.http.ListenAndServe()
}

func (s *Server) Stop() error {
	c := context.TODO()
	err := s.http.Shutdown(c)
	return err
}

func (s *Server) createHandler(p processor.Processor, successCallback func([]beat.Event)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logp.Debug("handler", "Request - URI: %s ; Method: %s", r.RequestURI, r.Method)

		if !checkSecretToken(r, s.config.SecretToken) {
			sendError(w, 401, "Invalid token", true)
			return
		}

		if r.Method != "POST" {
			sendError(w, 405, "Only post requests are supported", false)
			return
		}

		reader, err := decodeData(r)
		if err != nil {
			sendError(w, 400, fmt.Sprintf("Decoding error: %s", err.Error()), true)
			return
		}
		defer reader.Close()

		// Limit size of request to prevent for example zip bombs
		limitedReader := io.LimitReader(reader, s.config.MaxUnzippedSize)
		err = p.Validate(limitedReader)
		if err != nil {
			sendError(w, 400, fmt.Sprintf("Data Validation error: %s", err), true)
			return
		}

		list := p.Transform()

		w.WriteHeader(201)
		successCallback(list)
	}
}

func sendError(w http.ResponseWriter, code int, error string, log bool) {
	w.WriteHeader(code)
	w.Write([]byte(error))
	if log {
		logp.Err(error)
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
