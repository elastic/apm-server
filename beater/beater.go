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
}

// Creates beater
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig()
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

	bt.mutex.Lock()
	if bt.stopped {
		defer bt.mutex.Unlock()
		return nil
	}

	bt.server = newServer(bt.config, pub.Send)
	bt.mutex.Unlock()

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
	bt.mutex.Lock()
	if bt.server != nil {
		stop(bt.server, bt.config.ShutdownTimeout)
	}
	bt.stopped = true
	bt.mutex.Unlock()
}
