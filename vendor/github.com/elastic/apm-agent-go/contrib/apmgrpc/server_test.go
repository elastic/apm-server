package apmgrpc_test

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmgrpc"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func init() {
	// Register this test package as an application package so we can
	// check "culprit".
	type foo struct{}
	stacktrace.RegisterApplicationPackage(reflect.TypeOf(foo{}).PkgPath())
}

func TestServerTransaction(t *testing.T) {
	adaptTest := func(f func(*testing.T, testParams)) func(*testing.T) {
		return func(t *testing.T) {
			tracer, transport := transporttest.NewRecorderTracer()
			defer tracer.Close()

			s, server, addr := newServer(t, tracer)
			defer s.GracefulStop()

			conn, client := newClient(t, addr)
			defer conn.Close()

			f(t, testParams{
				server:    server,
				conn:      conn,
				client:    client,
				tracer:    tracer,
				transport: transport,
			})
		}
	}
	t.Run("happy", adaptTest(testServerTransactionHappy))
	t.Run("unknown_error", adaptTest(testServerTransactionUnknownError))
	t.Run("status_error", adaptTest(testServerTransactionStatusError))
	t.Run("panic", adaptTest(testServerTransactionPanic))
}

type testParams struct {
	server    *helloworldServer
	conn      *grpc.ClientConn
	client    pb.GreeterClient
	tracer    *elasticapm.Tracer
	transport *transporttest.RecorderTransport
}

func testServerTransactionHappy(t *testing.T, p testParams) {
	resp, err := p.client.SayHello(context.Background(), &pb.HelloRequest{
		Name: "birita",
	})
	require.NoError(t, err)
	assert.Equal(t, resp, &pb.HelloReply{Message: "hello, birita"})

	p.tracer.Flush(nil)
	tx := p.transport.Payloads()[0].Transactions()[0]
	assert.Equal(t, "/helloworld.Greeter/SayHello", tx.Name)
	assert.Equal(t, "grpc", tx.Type)
	assert.Equal(t, "OK", tx.Result)

	require.Len(t, tx.Context.Custom, 1)
	assert.Equal(t, "grpc", tx.Context.Custom[0].Key)
	grpcContext := tx.Context.Custom[0].Value.(map[string]interface{})
	assert.Contains(t, grpcContext, "peer.address")
	delete(grpcContext, "peer.address")
	assert.Equal(t, &model.Context{
		Custom: model.IfaceMap{{
			Key:   "grpc",
			Value: map[string]interface{}{},
		}},
	}, tx.Context)
}

func testServerTransactionUnknownError(t *testing.T, p testParams) {
	p.server.err = errors.New("boom")
	_, err := p.client.SayHello(context.Background(), &pb.HelloRequest{Name: "birita"})
	assert.EqualError(t, err, "rpc error: code = Unknown desc = boom")

	p.tracer.Flush(nil)
	payloads := p.transport.Payloads()
	require.Len(t, payloads, 1)
	tx := payloads[0].Transactions()[0]
	assert.Equal(t, "/helloworld.Greeter/SayHello", tx.Name)
	assert.Equal(t, "grpc", tx.Type)
	assert.Equal(t, "Unknown", tx.Result)
}

func testServerTransactionStatusError(t *testing.T, p testParams) {
	p.server.err = status.Errorf(codes.DataLoss, "boom")
	_, err := p.client.SayHello(context.Background(), &pb.HelloRequest{Name: "birita"})
	assert.EqualError(t, err, "rpc error: code = DataLoss desc = boom")

	p.tracer.Flush(nil)
	payloads := p.transport.Payloads()
	require.Len(t, payloads, 1)
	tx := payloads[0].Transactions()[0]
	assert.Equal(t, "/helloworld.Greeter/SayHello", tx.Name)
	assert.Equal(t, "grpc", tx.Type)
	assert.Equal(t, "DataLoss", tx.Result)
}

func testServerTransactionPanic(t *testing.T, p testParams) {
	p.server.panic = true
	p.server.err = errors.New("boom")
	_, err := p.client.SayHello(context.Background(), &pb.HelloRequest{Name: "birita"})
	assert.EqualError(t, err, "rpc error: code = Internal desc = boom")

	p.tracer.Flush(nil)
	payloads := p.transport.Payloads()
	require.Len(t, payloads, 2)
	e := payloads[0].Errors()[0]
	assert.NotEmpty(t, e.Transaction.ID)
	assert.Equal(t, false, e.Exception.Handled)
	assert.Equal(t, "(*helloworldServer).SayHello", e.Culprit)
	assert.Equal(t, "boom", e.Exception.Message)
}

func TestServerRecovery(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	s, server, addr := newServer(t, tracer, apmgrpc.WithRecovery())
	defer s.GracefulStop()

	conn, client := newClient(t, addr)
	defer conn.Close()

	server.panic = true
	server.err = errors.New("boom")
	_, err := client.SayHello(context.Background(), &pb.HelloRequest{Name: "birita"})
	assert.EqualError(t, err, "rpc error: code = Internal desc = boom")

	tracer.Flush(nil)
	payloads := transport.Payloads()
	require.Len(t, payloads, 2)
	e := payloads[0].Errors()[0]
	assert.NotEmpty(t, e.Transaction.ID)

	// Panic was recovered by the recovery interceptor and translated
	// into an Internal error.
	assert.Equal(t, true, e.Exception.Handled)
	assert.Equal(t, "(*helloworldServer).SayHello", e.Culprit)
	assert.Equal(t, "boom", e.Exception.Message)
}

func newServer(t *testing.T, tracer *elasticapm.Tracer, opts ...apmgrpc.ServerOption) (*grpc.Server, *helloworldServer, net.Addr) {
	// We always install grpc_recovery first to avoid panics
	// aborting the test process. We install it before the
	// apmgrpc interceptor so that apmgrpc can recover panics
	// itself if configured to do so.
	interceptors := []grpc.UnaryServerInterceptor{grpc_recovery.UnaryServerInterceptor()}
	serverOpts := []grpc.ServerOption{}
	if tracer != nil {
		opts = append(opts, apmgrpc.WithTracer(tracer))
		interceptors = append(interceptors, apmgrpc.NewUnaryServerInterceptor(opts...))
	}
	serverOpts = append(serverOpts, grpc_middleware.WithUnaryServerChain(interceptors...))

	s := grpc.NewServer(serverOpts...)
	server := &helloworldServer{}
	pb.RegisterGreeterServer(s, server)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	go s.Serve(lis)
	return s, server, lis.Addr()
}

func newClient(t *testing.T, addr net.Addr) (*grpc.ClientConn, pb.GreeterClient) {
	conn, err := grpc.Dial(
		addr.String(), grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor()),
	)
	require.NoError(t, err)
	return conn, pb.NewGreeterClient(conn)
}

type helloworldServer struct {
	panic bool
	err   error
}

func (s *helloworldServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	// The context passed to the server should contain a Transaction
	// for the gRPC request.
	span, ctx := elasticapm.StartSpan(ctx, "server_span", "type")
	if span != nil {
		span.Done(-1)
	}
	if s.panic {
		panic(s.err)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &pb.HelloReply{Message: "hello, " + req.Name}, nil
}
