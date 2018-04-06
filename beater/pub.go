package beater

import (
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	sysinfo "github.com/elastic/go-sysinfo"
)

// publisher forwards batches of events to libbeat. It uses GuaranteedSend
// to enable infinite retry of events being processed.
// If the publisher's input channel is full, an error is returned immediately.
// Number of concurrent requests waiting for processing do depend on the configured
// queue size. As the publisher is not waiting for the outputs ACK, the total
// number requests(events) active in the system can exceed the queue size. Only
// the number of concurrent HTTP requests trying to publish at the same time is limited.
type publisher struct {
	events  chan event
	client  beat.Client
	m       sync.RWMutex
	stopped bool
}

type event struct {
	payload pr.Payload
	config  config.Config
}

var (
	errFull              = errors.New("Queue is full")
	errInvalidBufferSize = errors.New("Request buffer must be > 0")
	errChannelClosed     = errors.New("Can't send batch, publisher is being stopped")
)

// newPublisher creates a new publisher instance.
//MaxCPU new go-routines are started for forwarding events to libbeat.
//Stop must be called to close the beat.Client and free resources.
func newPublisher(pipeline beat.Pipeline, N int, shutdownTimeout time.Duration) (*publisher, error) {
	if N <= 0 {
		return nil, errInvalidBufferSize
	}

	client, err := pipeline.ConnectWith(beat.ClientConfig{
		PublishMode: beat.GuaranteedSend,

		//       If set >0 `Close` will block for the duration or until pipeline is empty
		WaitClose:         shutdownTimeout,
		SkipNormalization: true,
	})
	if err != nil {
		return nil, err
	}

	p := &publisher{
		client: client,

		// Set channel size to N - 1. One request will be actively processed by the
		// worker, while the other concurrent requests will be buffered in the queue.
		events: make(chan event, N-1),
	}

	for i := 0; i < numCPUToUse(); i++ {
		go p.run()
	}

	return p, nil
}

func numCPUToUse() int {
	maxCPU := sysinfo.Go().MaxProcs
	numCPU := runtime.NumCPU()
	//https://golang.org/pkg/runtime/#GOMAXPROCS
	//If GOMAXPROCS < 1, it does not change the current setting.
	if maxCPU < 1 || maxCPU > numCPU {
		return numCPU
	}
	return maxCPU
}

// Stop closes all channels and waits for the the worker to stop.
// The worker will drain the queue on shutdown, but no more events
// will be published.
func (p *publisher) Stop() {
	p.m.Lock()
	p.stopped = true
	p.m.Unlock()
	close(p.events)
	p.client.Close()
}

// Send tries to forward events to the publishers worker. If the queue is full,
// an error is returned.
// Calling send after Stop will return an error.
func (p *publisher) Send(event event) error {
	p.m.RLock()
	defer p.m.RUnlock()
	if p.stopped {
		return errChannelClosed
	}

	select {
	case p.events <- event:
		return nil
	case <-time.After(time.Second * 1): // this forces the go scheduler to try something else for a while
		return errFull
	}
}

func (p *publisher) run() {
	for event := range p.events {
		p.client.PublishAll(event.payload.Transform(event.config))
	}
}
