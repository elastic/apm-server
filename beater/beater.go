package beater

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sync"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config  *Config
	mutex   sync.Mutex // guards server and stopped
	server  *http.Server
	stopped bool
	logger  *logp.Logger
}

// Creates beater
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig(b.Info.Version)
	if err := ucfg.Unpack(beaterConfig); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}
	if beaterConfig.Frontend.isEnabled() {
		if _, err := regexp.Compile(beaterConfig.Frontend.LibraryPattern); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `library_pattern`: %v", err.Error()))
		}
		if _, err := regexp.Compile(beaterConfig.Frontend.ExcludeFromGrouping); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `exclude_from_grouping`: %v", err.Error()))
		}
		if b.Config != nil && b.Config.Output.Name() == "elasticsearch" {
			beaterConfig.setElasticsearch(b.Config.Output.Config())
		}
	}

	bt := &beater{
		config:  beaterConfig,
		stopped: false,
		logger:  logp.NewLogger("beater"),
	}
	return bt, nil
}

func (bt *beater) Run(b *beat.Beat) error {
	var err error

	pub, err := newPublisher(b.Publisher, bt.config.ConcurrentRequests, bt.config.ShutdownTimeout)
	if err != nil {
		return err
	}
	defer pub.Stop()

	lis, err := net.Listen("tcp", bt.config.Host)
	if err != nil {
		bt.logger.Errorf("failed to listen: %s", err)
		return err
	}
	go notifyListening(bt.config, pub.Send)

	bt.mutex.Lock()
	if bt.stopped {
		defer bt.mutex.Unlock()
		return nil
	}

	bt.server = newServer(bt.config, pub.Send)
	bt.mutex.Unlock()

	err = run(bt.server, lis, bt.config)
	if err == http.ErrServerClosed {
		bt.logger.Infof("Listener stopped: %s", err.Error())
		return nil
	}
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	bt.logger.Infof("stopping apm-server... waiting maximum of %v seconds for queues to drain",
		bt.config.ShutdownTimeout.Seconds())
	bt.mutex.Lock()
	if bt.server != nil {
		stop(bt.server)
	}
	bt.stopped = true
	bt.mutex.Unlock()
}
