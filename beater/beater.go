package beater

import (
	"fmt"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config Config
	server *server
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

	pub, err := newPublisher(b.Publisher, bt.config.ConcurrentRequests)
	if err != nil {
		return err
	}
	defer pub.Stop()

	go notifyListening(bt.config, pub.Send)

	bt.server = newServer(bt.config, pub.Send)

	err = bt.server.run()
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	logp.Info("stopping apm-server...")
	bt.server.stop()
}
