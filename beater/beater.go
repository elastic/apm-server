package beater

import (
	"fmt"
	"net"
	"net/http"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config *Config
	server *http.Server
}

// Creates beater
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig()
	if err := ucfg.Unpack(beaterConfig); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	if b.Config != nil && b.Config.Output.Name() == "elasticsearch" {
		beaterConfig.setElasticsearch(b.Config.Output.Config())
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

	lis, err := net.Listen("tcp", bt.config.Host)
	if err != nil {
		logp.Err("failed to listen: %s", err)
		return err
	}
	go notifyListening(bt.config, pub.Send)

	bt.server = newServer(bt.config, pub.Send)

	err = run(bt.server, lis, bt.config)
	if err == http.ErrServerClosed {
		logp.Info("Listener stopped: %s", err.Error())
		return nil
	}
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	logp.Info("stopping apm-server...")
	stop(bt.server, bt.config.ShutdownTimeout)
}
