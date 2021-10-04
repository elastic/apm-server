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
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.elastic.co/apm/apmtest"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	_ "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"
	"github.com/elastic/beats/v7/libbeat/publisher/queue"
	"github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"

	"github.com/elastic/apm-server/model"
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

func BenchmarkPublisher(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})

	var indexed int64
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		scanner := bufio.NewScanner(r.Body)
		var n int64
		for scanner.Scan() {
			if scanner.Scan() {
				n++
			}
		}
		atomic.AddInt64(&indexed, n)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	supporter, err := idxmgmt.DefaultSupport(logp.NewLogger("beater_test"), beat.Info{}, nil)
	require.NoError(b, err)
	outputGroup, err := outputs.Load(supporter, beat.Info{}, nil, "elasticsearch", common.MustNewConfigFrom(map[string]interface{}{
		"hosts": []interface{}{srv.URL},
	}))
	require.NoError(b, err)
	pipeline, err := pipeline.New(
		beat.Info{},
		pipeline.Monitors{},
		func(lis queue.ACKListener) (queue.Queue, error) {
			return memqueue.NewQueue(nil, memqueue.Settings{
				ACKListener:    lis,
				FlushMinEvents: 2048,
				FlushTimeout:   time.Second,
				Events:         4096,
			}), nil
		},
		outputGroup,
		pipeline.Settings{
			WaitCloseMode:  pipeline.WaitOnClientClose,
			InputQueueSize: 2048,
		},
	)
	require.NoError(b, err)

	acker := publish.NewWaitPublishedAcker()
	acker.Open()
	publisher, err := publish.NewPublisher(
		pipetool.WithACKer(pipeline, acker),
		apmtest.DiscardTracer,
		&publish.PublisherConfig{},
	)
	require.NoError(b, err)

	batch := model.Batch{
		model.APMEvent{
			Processor: model.TransactionProcessor,
			Timestamp: time.Now(),
		},
	}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if err := publisher.ProcessBatch(ctx, &batch); err != nil {
			b.Fatal(err)
		}
	}

	// Close the publisher and wait for enqueued events to be published.
	assert.NoError(b, publisher.Stop(context.Background()))
	assert.NoError(b, acker.Wait(context.Background()))
	assert.NoError(b, pipeline.Close())
	assert.Equal(b, int64(b.N), indexed)
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
