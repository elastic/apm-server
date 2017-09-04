package beater

import (
	"context"
	"net/http"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	serverMetrics  = monitoring.Default.NewRegistry("apm-server.server")
	requestCounter = monitoring.NewInt(serverMetrics, "requests.counter")
	responseValid  = monitoring.NewInt(serverMetrics, "response.valid")
	responseErrors = monitoring.NewInt(serverMetrics, "response.errors")
)

type reporter func([]beat.Event) error

type server struct {
	http.Server
	config Config
}

func newServer(config Config, report reporter) *server {
	mux := http.NewServeMux()

	for path, p := range processor.Registry.Processors() {

		handler := createHandler(p, config, report)

		logp.Info("Path %s added to request handler", path)

		mux.HandleFunc(path, handler)
	}

	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		requestCounter.Inc()
		w.WriteHeader(200)
		responseValid.Inc()
	})

	logp.Info("Listening on: %s", config.Host)

	return &server{
		Server: http.Server{
			Addr:           config.Host,
			Handler:        mux,
			ReadTimeout:    config.ReadTimeout,
			WriteTimeout:   config.WriteTimeout,
			MaxHeaderBytes: config.MaxHeaderBytes,
		},
		config: config,
	}
}

func (s *server) run() error {
	logp.Info("starting apm-server! Hit CTRL-C to stop it.")

	var err error
	if s.config.SSL.isEnabled() {
		err = s.ListenAndServeTLS(s.config.SSL.Cert, s.config.SSL.PrivateKey)
	} else {
		err = s.ListenAndServe()
	}

	// http.ErrorServerClosed is the default error when the server is stopped.
	if err == http.ErrServerClosed {
		logp.Info("Listener stopped")
		return nil
	}
	return err
}

func (s *server) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		logp.Err(err.Error())
		err = s.Close()
		if err != nil {
			logp.Err(err.Error())
		}
	}
}
