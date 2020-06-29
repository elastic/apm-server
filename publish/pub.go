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
	"go.elastic.co/apm"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

type Reporter func(context.Context, PendingReq) error

// Publisher forwards batches of events to libbeat. It uses GuaranteedSend
// to enable infinite retry of events being processed.
// If the publisher's input channel is full, an error is returned immediately.
// Number of concurrent requests waiting for processing do depend on the configured
// queue size. As the publisher is not waiting for the outputs ACK, the total
// number requests(events) active in the system can exceed the queue size. Only
// the number of concurrent HTTP requests trying to publish at the same time is limited.
type Publisher struct {
	pendingRequests chan PendingReq
	tracer          *apm.Tracer
	client          beat.Client

	wg       sync.WaitGroup
	stopOnce sync.Once
	mu       sync.RWMutex
	stopped  bool
}

type PendingReq struct {
	Transformables []transform.Transformable
	Tcontext       *transform.Context
	Trace          bool
}

// PublisherConfig is a struct holding configuration information for the publisher,
// such as shutdown timeout, default pipeline name and beat info.
type PublisherConfig struct {
	Info            beat.Info
	ShutdownTimeout time.Duration
	Pipeline        string
}

var (
	ErrFull          = errors.New("queue is full")
	ErrChannelClosed = errors.New("can't send batch, publisher is being stopped")
)

// newPublisher creates a new publisher instance.
//MaxCPU new go-routines are started for forwarding events to libbeat.
//Stop must be called to close the beat.Client and free resources.
func NewPublisher(pipeline beat.Pipeline, tracer *apm.Tracer, cfg *PublisherConfig) (*Publisher, error) {
	processingCfg := beat.ProcessingConfig{
		Fields: common.MapStr{
			"observer": common.MapStr{
				"type":          cfg.Info.Beat,
				"hostname":      cfg.Info.Hostname,
				"version":       cfg.Info.Version,
				"version_major": 8,
				"id":            cfg.Info.ID.String(),
				"ephemeral_id":  cfg.Info.EphemeralID.String(),
			},
		},
	}
	if cfg.Pipeline != "" {
		processingCfg.Meta = map[string]interface{}{"pipeline": cfg.Pipeline}
	}
	client, err := pipeline.ConnectWith(beat.ClientConfig{
		PublishMode: beat.GuaranteedSend,
		// If set >0 `Close` will block for the duration or until pipeline is empty
		WaitClose:  cfg.ShutdownTimeout,
		Processing: processingCfg,
	})
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		tracer: tracer,
		client: client,

		// One request will be actively processed by the
		// worker, while the other concurrent requests will be buffered in the queue.
		pendingRequests: make(chan PendingReq, runtime.GOMAXPROCS(0)),
	}

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		p.wg.Add(1)
		go p.run()
	}

	return p, nil
}

// Client returns the beat client used by the publisher
func (p *Publisher) Client() beat.Client {
	return p.client
}

// Stop closes all channels and waits for the the worker to stop.
//
// The worker will drain the queue on shutdown, but no more requests
// will be published after Stop returns.
func (p *Publisher) Stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.stopped = true
		p.mu.Unlock()
		close(p.pendingRequests)
		// Wait for goroutines to stop before closing the client,
		// to ensure previously enqueued requests are published.
		p.wg.Wait()
		p.client.Close()
	})
}

// Send tries to forward pendingReq to the publishers worker. If the queue is full,
// an error is returned.
//
// Calling Send after Stop will return an error without enqueuing the request.
func (p *Publisher) Send(ctx context.Context, req PendingReq) error {
	if len(req.Transformables) == 0 {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
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

func (p *Publisher) run() {
	defer p.wg.Done()
	ctx := context.Background()
	for req := range p.pendingRequests {
		p.processPendingReq(ctx, req)
	}
}

func (p *Publisher) processPendingReq(ctx context.Context, req PendingReq) {
	var tx *apm.Transaction
	if req.Trace {
		tx = p.tracer.StartTransaction("ProcessPending", "Publisher")
		defer tx.End()
		ctx = apm.ContextWithTransaction(ctx, tx)
	}

	for _, transformable := range req.Transformables {
		events := transformTransformable(ctx, transformable, req.Tcontext)
		span := tx.StartSpan("PublishAll", "Publisher", nil)
		p.client.PublishAll(events)
		span.End()
	}
}

func transformTransformable(ctx context.Context, t transform.Transformable, tctx *transform.Context) []beat.Event {
	span, ctx := apm.StartSpan(ctx, "Transform", "Publisher")
	defer span.End()
	return t.Transform(ctx, tctx)
}
