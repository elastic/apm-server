package beater

import (
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/apm-server/onboarding"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config Config
	server *http.Server
	client beat.Client
}

// Creates beater
func New(_ *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig
	if err := ucfg.Unpack(&beaterConfig); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &beater{
		config: beaterConfig,
	}
	return bt, nil
}

func (bt *beater) Run(b *beat.Beat) error {

	var err error

	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}
	defer bt.client.Close()

	callback := func(events []beat.Event) {
		// Publishing does not wait for publishing to be acked
		go bt.client.PublishAll(events)
	}

	var checkServer = func() bool {
		secure := bt.config.SSL.isEnabled()
		return isServerUp(secure, bt.config.Host, 10, time.Second*6)
	}
	go onboarding.NotifyUp(checkServer, callback)

	bt.server = newServer(bt.config, callback)
	err = run(bt.server, bt.config.SSL)

	if err == http.ErrServerClosed {
		logp.Info(err.Error())
		return nil
	}
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	logp.Info("stopping apm-server...")
	stop(bt.server, bt.config.ShutdownTimeout)
}
