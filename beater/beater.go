package beater

import (
	"fmt"
	"net"
	"net/http"
	"regexp"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	es "github.com/elastic/beats/libbeat/outputs/elasticsearch"
)

type beater struct {
	config *Config
	server *http.Server
}

// Creates beater
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig()
	if err := ucfg.Unpack(beaterConfig); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	if err := validateConfig(beaterConfig, b.Config); err != nil {
		return nil, err
	}

	if beaterConfig.Frontend.isEnabled() {
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


func validateConfig(beaterConfig *Config, beatConfig *beat.BeatConfig) error {
	if beaterConfig.Frontend.isEnabled() {
		if _, err := regexp.Compile(beaterConfig.Frontend.LibraryPattern); err != nil {
			return fmt.Errorf("invalid regex for `library_pattern`: %v", err)
		}
		if _, err := regexp.Compile(beaterConfig.Frontend.ExcludeFromGrouping); err != nil {
			return fmt.Errorf("invalid regex for `exclude_from_grouping`: %v", err)
		}
	}
	if beatConfig != nil && beatConfig.Output.Name() == "elasticsearch" {
		clients, err := es.NewElasticsearchClients(beatConfig.Output.Config())
		if err != nil {
			return fmt.Errorf("elasticsearch not available: %v", err)
		}
		for _, client := range clients {
			if err := client.Connect(); err != nil {
				return fmt.Errorf("elasticsearch not available: %v", err)
			}
		}
	}
	return nil
}
