package beater

import (
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"

	"math"

	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/beats/libbeat/beat"
	"fmt"
)

// Publisher forwards batches of events to libbeat. It uses GuaranteedSend
// to enable infinite retry of events being processed.
// If the publisher's input channel is full, an error is returned immediately.
// Number of concurrent requests waiting for processing do depend on the configured
// queue size. As the publisher is not waiting for the outputs ACK, the total
// number requests(events) active in the system can exceed the queue size. Only
// the number of concurrent HTTP requests trying to publish at the same time is limited.
type publisher struct {
	errorsC       chan pendingReq
	transactionsC chan pendingReq
	sourcemapsC   chan pendingReq
	client        beat.Client
	m             sync.RWMutex
	stopped       bool
}

type pendingReq struct {
	processorName string
	payload       pr.Payload
	config        config.Config
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
	transactionsCSize := N - 1
	errorsCSize := int(math.Max(math.Trunc(float64(transactionsCSize) * 0.2), 1))
	fmt.Println("transSize  ", transactionsCSize)
	fmt.Println("errSize ", errorsCSize)
	p := &publisher{
		client:        client,
		transactionsC: make(chan pendingReq, transactionsCSize),
		errorsC:       make(chan pendingReq, errorsCSize),
		sourcemapsC:   make(chan pendingReq, 1),
	}

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go p.run()
	}

	return p, nil
}

// Stop closes all channels and waits for the the worker to stop.
// The worker will drain the queue on shutdown, but no more pending requests
// will be published.
func (p *publisher) Stop() {
	p.m.Lock()
	p.stopped = true
	p.m.Unlock()
	close(p.transactionsC)
	close(p.errorsC)
	close(p.sourcemapsC)
	p.client.Close()
}

// Send tries to forward pendingReq to the publishers worker. If the queue is full,
// an error is returned.
// Calling send after Stop will return an error.
func (p *publisher) Send(req pendingReq) error {
	p.m.RLock()
	defer p.m.RUnlock()
	if p.stopped {
		return errChannelClosed
	}
	switch req.processorName {
	case transaction.ProcessorName:
		select {
		case p.transactionsC <- req:
			return nil
		case <-time.After(time.Second * 1):
			return errFull
		}

	case perr.ProcessorName:
		select {
		case p.errorsC <- req:
			return nil
		case <-time.After(time.Second * 1):
			return errFull
		}

	case sourcemap.ProcessorName:
		p.sourcemapsC <- req
	}
	return nil
}

func (p *publisher) run() {
	pendingRequests := make(chan pendingReq)
	go func() {
		defer close(pendingRequests)
		var ok = true
		for ok {
			var req pendingReq
			select {
			case req, ok = <- p.transactionsC:
			case req, ok = <- p.errorsC:
			case req, ok = <- p.sourcemapsC:
			}
			if ok {
				pendingRequests <- req
			}
		}
	}()
	for req := range pendingRequests {
		p.client.PublishAll(req.payload.Transform(req.config))
	}
}
