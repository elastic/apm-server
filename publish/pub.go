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
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/beat"
)

type Reporter func(context.Context, PendingReq) error

// Publisher forwards batches of events to libbeat. It uses GuaranteedSend
// to enable infinite retry of events being processed.
// If the publisher's input channel is full, an error is returned immediately.
// Number of concurrent requests waiting for processing do depend on the configured
// queue size. As the publisher is not waiting for the outputs ACK, the total
// number requests(events) active in the system can exceed the queue size. Only
// the number of concurrent HTTP requests trying to publish at the same time is limited.
type publisher struct {
	pendingRequests chan PendingReq
	tracer          *elasticapm.Tracer
	client          beat.Client
	m               sync.RWMutex
	stopped         bool
}

type PendingReq struct {
	Transformables []transform.Transformable
	Tcontext       *transform.Context
	Trace          bool
}

var (
	ErrFull              = errors.New("queue is full")
	ErrInvalidBufferSize = errors.New("request buffer must be > 0")
	ErrChannelClosed     = errors.New("can't send batch, publisher is being stopped")
)

// newPublisher creates a new publisher instance.
//MaxCPU new go-routines are started for forwarding events to libbeat.
//Stop must be called to close the beat.Client and free resources.
func NewPublisher(pipeline beat.Pipeline, N int, shutdownTimeout time.Duration, tracer *elasticapm.Tracer) (*publisher, error) {
	if N <= 0 {
		return nil, ErrInvalidBufferSize
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
		tracer: tracer,
		client: client,

		// Set channel size to N - 1. One request will be actively processed by the
		// worker, while the other concurrent requests will be buffered in the queue.
		pendingRequests: make(chan PendingReq, N-1),
	}

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go p.run()
	}

	return p, nil
}

func (p *publisher) Client() beat.Client {
	return p.client
}

// Stop closes all channels and waits for the the worker to stop.
// The worker will drain the queue on shutdown, but no more pending requests
// will be published.
func (p *publisher) Stop() {
	p.m.Lock()
	p.stopped = true
	p.m.Unlock()
	close(p.pendingRequests)
	p.client.Close()
}

// Send tries to forward pendingReq to the publishers worker. If the queue is full,
// an error is returned.
// Calling send after Stop will return an error.
func (p *publisher) Send(ctx context.Context, req PendingReq) error {
	p.m.RLock()
	defer p.m.RUnlock()
	if p.stopped {
		return ErrChannelClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.pendingRequests <- req:
		return nil
	case <-time.After(time.Second * 1): // this forces the go scheduler to try something else for a while
		return ErrFull
	}
}

func (p *publisher) run() {
	for req := range p.pendingRequests {
		p.processPendingReq(req)
	}
}

func (p *publisher) processPendingReq(req PendingReq) {
	var tx *elasticapm.Transaction
	if req.Trace {
		tx = p.tracer.StartTransaction("ProcessPending", "Publisher")
		defer tx.End()
	}

	for _, transformable := range req.Transformables {
		span := tx.StartSpan("Transform", "Publisher", nil)
		events := transformable.Transform(req.Tcontext)
		span.End()

		span = tx.StartSpan("PublishAll", "Publisher", nil)
		p.client.PublishAll(events)
		span.End()
	}
}
