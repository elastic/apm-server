package apmgrpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestClientSpan(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	s, _, addr := newServer(t, nil) // no server tracing
	defer s.GracefulStop()

	conn, client := newClient(t, addr)
	defer conn.Close()
	resp, err := client.SayHello(context.Background(), &pb.HelloRequest{Name: "birita"})
	require.NoError(t, err)
	assert.Equal(t, resp, &pb.HelloReply{Message: "hello, birita"})

	// The client interceptor starts no transactions, only spans.
	tracer.Flush(nil)
	require.Len(t, transport.Payloads(), 0)

	tx := tracer.StartTransaction("name", "type")
	ctx := elasticapm.ContextWithTransaction(context.Background(), tx)
	resp, err = client.SayHello(ctx, &pb.HelloRequest{Name: "birita"})
	require.NoError(t, err)
	assert.Equal(t, resp, &pb.HelloReply{Message: "hello, birita"})
	tx.Done(-1)

	tracer.Flush(nil)
	out := transport.Payloads()[0].Transactions()[0]
	require.Len(t, out.Spans, 1)
	assert.Equal(t, "/helloworld.Greeter/SayHello", out.Spans[0].Name)
}
