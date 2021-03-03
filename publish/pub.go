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

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
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
	stopped         chan struct{}
	tracer          *apm.Tracer
	client          beat.Client
	transformConfig *transform.Config

	mu              sync.RWMutex
	stopping        bool
	pendingRequests chan PendingReq
}

type PendingReq struct {
	Transformable transform.Transformable
	Trace         bool
}

// PublisherConfig is a struct holding configuration information for the publisher.
type PublisherConfig struct {
	Info            beat.Info
	Pipeline        string
	Namespace       string
	Processor       beat.ProcessorList
	TransformConfig *transform.Config
}

func (cfg *PublisherConfig) Validate() error {
	if cfg.TransformConfig == nil {
		return errors.New("TransfromConfig unspecified")
	}
	return nil
}

var (
	ErrFull          = errors.New("queue is full")
	ErrChannelClosed = errors.New("can't send batch, publisher is being stopped")
)

// newPublisher creates a new publisher instance.
//
// GOMAXPROCS goroutines are started for forwarding events to libbeat.
// Stop must be called to close the beat.Client and free resources.
func NewPublisher(pipeline beat.Pipeline, tracer *apm.Tracer, cfg *PublisherConfig) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}

	observerFields := common.MapStr{
		"type":         cfg.Info.Beat,
		"hostname":     cfg.Info.Hostname,
		"version":      cfg.Info.Version,
		"id":           cfg.Info.ID.String(),
		"ephemeral_id": cfg.Info.EphemeralID.String(),
	}
	if version, err := common.NewVersion(cfg.Info.Version); err == nil {
		observerFields["version_major"] = version.Major
	}

	processingCfg := beat.ProcessingConfig{
		Fields:    common.MapStr{"observer": observerFields},
		Processor: cfg.Processor,
	}
	if cfg.TransformConfig.DataStreams {
		processingCfg.Fields[datastreams.NamespaceField] = cfg.Namespace
	}
	if cfg.Pipeline != "" {
		processingCfg.Meta = map[string]interface{}{"pipeline": cfg.Pipeline}
	}

	p := &Publisher{
		tracer:          tracer,
		stopped:         make(chan struct{}),
		transformConfig: cfg.TransformConfig,

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
	events := transformTransformable(ctx, req.Transformable, p.transformConfig)
	span := tx.StartSpan("PublishAll", "Publisher", nil)
	defer span.End()
	p.client.PublishAll(events)
}

func transformTransformable(ctx context.Context, transformable transform.Transformable, cfg *transform.Config) []beat.Event {
	span, ctx := apm.StartSpan(ctx, "Transform", "Publisher")
	defer span.End()
	return transformable.Transform(ctx, cfg)
}
