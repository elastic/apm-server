package beater

import (
	"context"
	"net/http"
	"time"

	"github.com/elastic/apm-server/version"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
)

type reporter func([]beat.Event) error

func newServer(config Config, report reporter) *http.Server {
	mux := newMuxer(config, report)

	return &http.Server{
		Addr:           config.Host,
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
}

func run(server *http.Server, config Config) error {
	logp.Info("Starting apm-server [%s]. Hit CTRL-C to stop it.", version.String())
	logp.Info("Listening on: %s", server.Addr)
	switch config.Frontend.isEnabled() {
	case true:
		logp.Info("Frontend endpoints enabled!")
	case false:
		logp.Info("Frontend endpoints disabled")
	}

	ssl := config.SSL
	if ssl.isEnabled() {
		return server.ListenAndServeTLS(ssl.Cert, ssl.PrivateKey)
	}
	if config.SecretToken != "" {
		logp.Warn("Secret token is set, but SSL is not enabled.")
	}
	return server.ListenAndServe()
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
