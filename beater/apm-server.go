package beater

import (
	"fmt"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/server"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	publisher "github.com/elastic/beats/libbeat/publisher/beat"
)

type ApmServer struct {
	done   chan struct{}
	server *server.Server
	config config.Config
	client publisher.Client
}

// Creates beater
func New(_ *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	config := config.DefaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &ApmServer{
		done:   make(chan struct{}),
		config: config,
	}
	return bt, nil
}

func (bt *ApmServer) Run(b *beat.Beat) error {
	logp.Info("apm-server is running! Hit CTRL-C to stop it.")
	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}
	defer bt.client.Close()

	callback := func(events []publisher.Event) {
		// Publishing does not wait for publishing to be acked
		go bt.client.PublishAll(events)
	}

	bt.server, err = server.New(bt.config.Server)
	if err != nil {
		return err
	}
	bt.server.Start(callback, bt.config.Host)
	defer bt.server.Stop()

	// Blocks until service is shut down
	<-bt.done

	return nil
}

func (bt *ApmServer) Stop() {
	close(bt.done)
}
