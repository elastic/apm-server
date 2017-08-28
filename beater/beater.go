package beater

import (
	"errors"
	"fmt"
	"sync"

	"net/http"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
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
	eventsLeft int
	dropped    bool
	done       chan bool
}

func (e *Eventer) Closing() {}
func (e *Eventer) Closed()  {}

func (e *Eventer) checkIfDone() {
	if e.eventsLeft == 0 {
		e.done <- true
	}
}

func (e *Eventer) reset(eventsLeft int) {
	e.dropped = false
	e.eventsLeft = eventsLeft
}

func (e *Eventer) Published() {
	e.eventsLeft--
	e.checkIfDone()
}

func (e *Eventer) FilteredOut(event beat.Event) {
	e.eventsLeft--
	e.checkIfDone()
}

func (e *Eventer) DroppedOnPublish(event beat.Event) {
	e.eventsLeft--
	e.dropped = true
	e.checkIfDone()
}

func (bt *beater) Run(b *beat.Beat) error {
	var mu sync.Mutex

	done := make(chan bool)
	eventer := Eventer{
		eventsLeft: 0,
		done:       done,
	}

	client, err := b.Publisher.ConnectWith(beat.ClientConfig{
		PublishMode: beat.DropIfFull,
		Events:      &eventer,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	callback := func(events []beat.Event) error {
		mu.Lock()
		defer mu.Unlock()

		eventer.reset(len(events))

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
	err = run(bt.server, bt.config.SSL)
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
