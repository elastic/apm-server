package beater

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
)

type reporter func([]beat.Event) error

func newServer(config *Config, report reporter) *http.Server {
	mux := newMuxer(config, report)

	return &http.Server{
		Addr:           config.Host,
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderSize,
	}
}

func run(server *http.Server, lis net.Listener, config *Config) error {
	logger := logp.NewLogger("server")
	logger.Infof("Starting apm-server. Hit CTRL-C to stop it.")
	logger.Infof("Listening on: %s", server.Addr)
	switch config.Frontend.isEnabled() {
	case true:
		logger.Info("Frontend endpoints enabled!")
	case false:
		logger.Info("Frontend endpoints disabled")
	}

	ssl := config.SSL
	if ssl.isEnabled() {
		return server.ServeTLS(lis, ssl.Cert, ssl.PrivateKey)
	}
	if config.SecretToken != "" {
		logger.Warn("Secret token is set, but SSL is not enabled.")
	}
	return server.Serve(lis)
}

func stop(server *http.Server, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger := logp.NewLogger("server")
	err := server.Shutdown(ctx)
	if err != nil {
		logger.Error(err.Error())
		err = server.Close()
		if err != nil {
			logger.Error(err.Error())
		}
	}
}
