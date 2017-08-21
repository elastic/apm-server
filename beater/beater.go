package beater

import (
	"fmt"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	done   chan struct{}
	config Config
	client beat.Client
}

// Creates beater
func New(_ *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig
	if err := ucfg.Unpack(&beaterConfig); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &beater{
		done:   make(chan struct{}),
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

	server := newServer(bt.config, callback)
	start(server, bt.config.SSL)
	defer stop(server)

	logp.Info("apm-server is running! Hit CTRL-C to stop it.")
	// Blocks until service is shut down
	<-bt.done

	return nil
}

func (bt *beater) Stop() {
	close(bt.done)
}
