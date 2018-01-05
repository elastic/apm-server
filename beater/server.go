package beater

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/elastic/apm-server/version"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
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
	switch config.Frontend.isEnabled() {
	case true:
		logp.Info("Frontend endpoints enabled!")
	case false:
		logp.Info("Frontend endpoints disabled")
	}

	ssl := config.SSL
	if ssl.isEnabled() {
		logp.Info("Loading server certificate: %s", config.SSL.Certificate.Certificate)
		logp.Info("Loading private key: %s", config.SSL.Certificate.Key)
		cert, err := outputs.LoadCertificate(&config.SSL.Certificate)
		if err != nil {
			return err
		}

		//Injecting certificate to the http.Server instead of passing the cert and key paths.
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*cert}}
		logp.Info("Listening on: https://%s", server.Addr)
		return server.ListenAndServeTLS("", "")
	}
	if config.SecretToken != "" {
		logp.Warn("Secret token is set, but SSL is not enabled.")
	}
	logp.Info("Listening on: http://%s", server.Addr)
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
