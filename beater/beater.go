package beater

import (
	"errors"
	"fmt"

	"net/http"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/atomic"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config Config
	server *http.Server
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

type Eventer struct {
	eventsToBeQueued atomic.Int32
	eventsToBeAcked  atomic.Int32
	dropped          bool
	done             chan bool
}

func (e *Eventer) Closing() {}
func (e *Eventer) Closed()  {}

func (e *Eventer) checkIfDone() {
	if e.eventsToBeQueued.Load() == 0 {
		e.done <- true
	}
}

func (e *Eventer) Published() {
	e.eventsToBeQueued.Dec()
	e.checkIfDone()
}

func (e *Eventer) FilteredOut(event beat.Event) {
	e.eventsToBeQueued.Dec()
	e.checkIfDone()
}

func (e *Eventer) DroppedOnPublish(event beat.Event) {
	e.eventsToBeQueued.Dec()
	e.dropped = true
	e.checkIfDone()
}

func (bt *beater) Run(b *beat.Beat) error {
	callback := func(events []beat.Event) error {
		eventer := Eventer{
			eventsToBeQueued: atomic.MakeInt32(int32(len(events))),
			done:             make(chan bool, 1),
			dropped:          false,
		}

		var client beat.Client
		client, err := b.Publisher.ConnectWith(beat.ClientConfig{
			PublishMode: beat.DropIfFull,
			Events:      &eventer,
		})
		defer client.Close()
		if err != nil {
			return err
		}

		go client.PublishAll(events)

		select {
		case <-eventer.done:
			if eventer.dropped {
				return errors.New("Queue is full")
			}
			return nil
		}
	}

	bt.server = newServer(bt.config, callback)
	err := run(bt.server, bt.config.SSL)
	logp.Err(err.Error())

	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	logp.Info("stopping apm-server...")
	stop(bt.server, bt.config.ShutdownTimeout)
}
