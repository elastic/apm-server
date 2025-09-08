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
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.elastic.co/fastjson"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/outputs"
	_ "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/mapstr"

	"github.com/elastic/apm-data/model/modelpb"

	"github.com/elastic/apm-server/internal/publish"
)

func TestPublisherStop(t *testing.T) {
	// Create a pipeline with a limited queue size and no outputs,
	// so we can simulate a pipeline that blocks indefinitely.
	pipeline, client := newBlockingPipeline(t)
	publisher, err := publish.NewPublisher(pipeline)
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
			Transformable: makeTransformable(beat.Event{Fields: make(mapstr.M)}),
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

	// Unblock the output, which should unblock publisher.Stop.
	close(client.unblock)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, publisher.Stop(ctx))
}

func TestPublisherStopShutdownInactive(t *testing.T) {
	pipeline, _ := newBlockingPipeline(t)
	publisher, err := publish.NewPublisher(pipeline)
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

	var indexed atomic.Int64
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		gzr, err := gzip.NewReader(r.Body)
		assert.NoError(b, err)
		defer gzr.Close()

		var jsonw fastjson.Writer
		jsonw.RawString(`{"items":[`)
		first := true

		scanner := bufio.NewScanner(gzr)
		var n int64

		// stop if there's no more data or we bump into an empty line
		// Prevent an issue with clients appending newlines to
		// valid requests
		for scanner.Scan() && len(scanner.Bytes()) != 0 { // index
			require.True(b, scanner.Scan())

			if first {
				first = false
			} else {
				jsonw.RawByte(',')
			}
			jsonw.RawString(`{"create":{"status":201}}`)
			n++
		}
		assert.NoError(b, scanner.Err())
		jsonw.RawString(`]}`)
		w.Write(jsonw.Bytes())
		indexed.Add(n)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	supporter, err := idxmgmt.DefaultSupport(
		beat.Info{
			Logger: logp.NewNopLogger(),
		},
		nil,
	)
	require.NoError(b, err)

	outputGroup, err := outputs.Load(
		supporter,
		beat.Info{
			Logger: logp.NewNopLogger(),
		},
		nil,
		"elasticsearch",
		config.MustNewConfigFrom(map[string]interface{}{
			"hosts": []interface{}{srv.URL},
		}),
	)
	require.NoError(b, err)

	conf, err := config.NewConfigFrom(map[string]interface{}{
		"mem.events":           4096,
		"mem.flush.min_events": 2048,
		"mem.flush.timeout":    "1s",
	})
	require.NoError(b, err)

	namespace := config.Namespace{}
	err = conf.Unpack(&namespace)
	require.NoError(b, err)

	pipeline, err := pipeline.New(
		beat.Info{
			Logger: logp.NewNopLogger(),
		},
		pipeline.Monitors{
			Logger: logp.NewNopLogger(),
		},
		namespace,
		outputGroup,
		pipeline.Settings{
			WaitCloseMode:  pipeline.WaitOnPipelineClose,
			InputQueueSize: 2048,
		},
	)
	require.NoError(b, err)

	acker := publish.NewWaitPublishedAcker()
	acker.Open()
	publisher, err := publish.NewPublisher(
		pipetool.WithACKer(pipeline, acker),
	)
	require.NoError(b, err)

	batch := modelpb.Batch{
		&modelpb.APMEvent{
			Timestamp: modelpb.FromTime(time.Now()),
		},
	}
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := publisher.ProcessBatch(ctx, &batch); err != nil {
				b.Fatal(err)
			}
		}
	})

	// Close the publisher and wait for enqueued events to be published.
	assert.NoError(b, publisher.Stop(context.Background()))
	assert.NoError(b, acker.Wait(context.Background()))
	assert.NoError(b, pipeline.Close())
	assert.Equal(b, int64(b.N), indexed.Load())
}

func newBlockingPipeline(t testing.TB) (*pipeline.Pipeline, *mockClient) {
	client := &mockClient{unblock: make(chan struct{})}
	conf, err := config.NewConfigFrom(map[string]interface{}{
		"mem.events":           32,
		"mem.flush.min_events": 1,
	})
	require.NoError(t, err)
	namespace := config.Namespace{}
	err = conf.Unpack(&namespace)
	require.NoError(t, err)

	pipeline, err := pipeline.New(
		beat.Info{
			Logger: logptest.NewTestingLogger(t, "beat"),
		},
		pipeline.Monitors{},
		namespace,
		outputs.Group{Clients: []outputs.Client{client}},
		pipeline.Settings{},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pipeline.Close())
	})
	return pipeline, client
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

type mockClient struct {
	unblock chan struct{}
}

func (c *mockClient) String() string { return "mock_client" }
func (c *mockClient) Close() error   { return nil }
func (c *mockClient) Publish(ctx context.Context, batch publisher.Batch) error {
	select {
	case <-c.unblock:
	case <-ctx.Done():
		return ctx.Err()
	}
	batch.ACK()
	return nil
}
