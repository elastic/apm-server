// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package publish

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.elastic.co/apm/v2"
	"go.elastic.co/fastjson"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-data/model/modeljson"
	"github.com/elastic/apm-data/model/modelpb"
)

type Reporter func(context.Context, PendingReq) error

// Publisher forwards batches of events to libbeat.
//
// If the publisher's input channel is full, an error is returned immediately.
// Publisher uses GuaranteedSend to enable infinite retry of events being processed.
//
// The number of concurrent requests waiting for processing depends on the configured
// queue size in libbeat. As the publisher is not waiting for the outputs ACK, the total
// number of events active in the system can exceed the queue size. Only the number of
// concurrent HTTP requests trying to publish at the same time is limited.
type Publisher struct {
	stopped chan struct{}
	tracer  *apm.Tracer
	client  beat.Client

	mu              sync.RWMutex
	stopping        bool
	pendingRequests chan PendingReq
}

type PendingReq struct {
	Transformable Transformer
}

// Transformer is an interface implemented by types that can be transformed into beat.Events.
type Transformer interface {
	Transform(context.Context) []beat.Event
}

var (
	ErrFull          = errors.New("queue is full")
	ErrChannelClosed = errors.New("can't send batch, publisher is being stopped")
)

// NewPublisher creates a new publisher instance.
//
// GOMAXPROCS goroutines are started for forwarding events to libbeat.
// Stop must be called to close the beat.Client and free resources.
func NewPublisher(pipeline beat.Pipeline, tracer *apm.Tracer) (*Publisher, error) {
	processingCfg := beat.ProcessingConfig{}
	p := &Publisher{
		tracer:  tracer,
		stopped: make(chan struct{}),

		// One request will be actively processed by the
		// worker, while the other concurrent requests will be buffered in the queue.
		pendingRequests: make(chan PendingReq, runtime.GOMAXPROCS(0)),
	}

	client, err := pipeline.ConnectWith(beat.ClientConfig{
		PublishMode: beat.GuaranteedSend,
		Processing:  processingCfg,
	})
	if err != nil {
		return nil, err
	}
	p.client = client

	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.run()
		}()
	}
	go func() {
		defer close(p.stopped)
		wg.Wait()
	}()

	return p, nil
}

// Stop closes all channels and waits for the the worker to stop, or for the
// context to be signalled. If the context is never cancelled, Stop may block
// indefinitely.
//
// The worker will drain the queue on shutdown, but no more requests will be
// published after Stop returns. Events may still exist in the libbeat pipeline
// after Stop returns; the caller is responsible for installing an ACKer as
// necessary.
func (p *Publisher) Stop(ctx context.Context) error {
	// Prevent additional requests from being enqueued.
	p.mu.Lock()
	if !p.stopping {
		p.stopping = true
		close(p.pendingRequests)
	}
	p.mu.Unlock()

	// Wait for enqueued events to be published. Order of events is
	// important here:
	//   (1) wait for pendingRequests to be drained and published (p.stopped)
	//   (2) close the beat.Client to prevent more events being published
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.stopped:
	}
	return p.client.Close()
}

// ProcessBatch transforms batch to beat.Events, and sends them to the libbeat
// publishing pipeline.
func (p *Publisher) ProcessBatch(ctx context.Context, batch *modelpb.Batch) error {
	b := make(modelpb.Batch, len(*batch))
	for i, e := range *batch {
		cp := e.CloneVT()
		b[i] = cp
	}
	return p.Send(ctx, PendingReq{Transformable: batchTransformer(b)})
}

// Send tries to forward pendingReq to the publishers worker. If the queue is full,
// an error is returned.
//
// Calling Send after Stop will return an error without enqueuing the request.
func (p *Publisher) Send(ctx context.Context, req PendingReq) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stopping {
		return ErrChannelClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.pendingRequests <- req:
		return nil
	case <-time.After(time.Second * 1):
		// TODO(axw) instead of having an arbitrary delay here,
		// make it the caller's responsibility to use a context
		// with a timeout.
		return ErrFull
	}
}

func (p *Publisher) run() {
	ctx := context.Background()
	for req := range p.pendingRequests {
		events := req.Transformable.Transform(ctx)
		p.client.PublishAll(events)
	}
}

type batchTransformer modelpb.Batch

func (t batchTransformer) Transform(context.Context) []beat.Event {
	out := make([]beat.Event, 0, len(t))
	var w fastjson.Writer
	for _, event := range t {
		// Encode the event to JSON, then decode into a map.
		// This is probably a bit horrifying, but enables us to
		// remove the libbeat dependency from our data model.
		//
		// This code path exists only for lesser-used outputs
		// that travel through libbeat. We mitigate the overhead
		// of the extra encode/decode by reusing a memory buffer
		// within a batch. This will also enable us to move away
		// from the intermediate map representation for events,
		// and encode directly to JSON, minimising garbage for
		// the Elasticsearch output.
		if err := modeljson.MarshalAPMEvent(event, &w); err != nil {
			continue
		}
		beatEvent := beat.Event{Timestamp: modelpb.ToTime(event.Timestamp)}
		if err := json.Unmarshal(w.Bytes(), &beatEvent.Fields); err != nil {
			continue
		}
		out = append(out, beatEvent)
		w.Reset()
	}
	return out
}
