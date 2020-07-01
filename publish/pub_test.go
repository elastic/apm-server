package publish_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/queue"
	"github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"
	"go.elastic.co/apm/apmtest"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func TestPublisherStop(t *testing.T) {
	// Create a pipeline with a limited queue size and no outputs,
	// so we can simulate a pipeline that blocks indefinitely.
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
			Transformables: []transform.Transformable{makeTransformable(
				beat.Event{Fields: make(common.MapStr)},
			)},
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

func makeTransformable(events ...beat.Event) transform.Transformable {
	return transformableFunc(func(ctx context.Context, tctx *transform.Context) []beat.Event {
		return events
	})
}

type transformableFunc func(context.Context, *transform.Context) []beat.Event

func (f transformableFunc) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	return f(ctx, tctx)
}

type mockClient struct{}

func (*mockClient) String() string { return "mock_client" }
func (*mockClient) Close() error   { return nil }
func (*mockClient) Publish(_ context.Context, batch publisher.Batch) error {
	batch.ACK()
	return nil
}
