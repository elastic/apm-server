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

package publish_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.elastic.co/apm/apmtest"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/queue"
	"github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"

	"github.com/elastic/apm-server/publish"
)

func TestPublisherStop(t *testing.T) {
	// Create a pipeline with a limited queue size and no outputs,
	// so we can simulate a pipeline that blocks indefinitely.
	pipeline := newBlockingPipeline(t)
	publisher, err := publish.NewPublisher(
		pipeline, apmtest.DiscardTracer, &publish.PublisherConfig{},
	)
	require.NoError(t, err)
	defer func() {
		cancelledContext, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately to avoid blocking Stop
		publisher.Stop(cancelledContext)
	}()

	// Publish events until the publisher's channel fills up, meaning that
	// the pipeline is full and there are still pending requests. The publisher
	// waits for enqueued requests to be published, up until a configurable
	// time has elapsed.
	for {
		err := publisher.Send(context.Background(), publish.PendingReq{
			Transformable: makeTransformable(beat.Event{Fields: make(common.MapStr)}),
		})
		if err == publish.ErrFull {
			break
		}
		assert.NoError(t, err)
	}

	// Stopping the publisher should block until the context is cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.Equal(t, context.DeadlineExceeded, publisher.Stop(ctx))

	// Set an output which acknowledges events immediately, unblocking publisher.Stop.
	assert.NoError(t, pipeline.OutputReloader().Reload(nil,
		func(outputs.Observer, common.ConfigNamespace) (outputs.Group, error) {
			return outputs.Group{Clients: []outputs.Client{&mockClient{}}}, nil
		},
	))
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, publisher.Stop(ctx))
}

func TestPublisherStopShutdownInactive(t *testing.T) {
	publisher, err := publish.NewPublisher(
		newBlockingPipeline(t),
		apmtest.DiscardTracer,
		&publish.PublisherConfig{},
	)
	require.NoError(t, err)

	// There are no active events, so the publisher should stop immediately
	// and not wait for the context to be cancelled.
	assert.NoError(t, publisher.Stop(context.Background()))
}

func newBlockingPipeline(t testing.TB) *pipeline.Pipeline {
	pipeline, err := pipeline.New(
		beat.Info{},
		pipeline.Monitors{},
		func(lis queue.ACKListener) (queue.Queue, error) {
			return memqueue.NewQueue(nil, memqueue.Settings{
				ACKListener: lis,
				Events:      1,
			}), nil
		},
		outputs.Group{},
		pipeline.Settings{},
	)
	require.NoError(t, err)
	return pipeline
}

func makeTransformable(events ...beat.Event) publish.Transformer {
	return transformableFunc(func(ctx context.Context) []beat.Event {
		return events
	})
}

type transformableFunc func(context.Context) []beat.Event

func (f transformableFunc) Transform(ctx context.Context) []beat.Event {
	return f(ctx)
}

type mockClient struct{}

func (*mockClient) String() string { return "mock_client" }
func (*mockClient) Close() error   { return nil }
func (*mockClient) Publish(_ context.Context, batch publisher.Batch) error {
	batch.ACK()
	return nil
}
