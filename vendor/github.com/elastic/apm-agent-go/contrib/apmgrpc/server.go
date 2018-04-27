package apmgrpc

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-agent-go"
)

// NewUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that
// traces gRPC requests with the given options.
//
// The interceptor will trace transactions with the "grpc" type for each
// incoming request. The transaction will be added to the context, so
// server methods can use elasticapm.StartSpan with the provided context.
//
// By default, the interceptor will trace with elasticapm.DefaultTracer,
// and will not recover any panics. Use WithTracer to specify an
// alternative tracer, and WithRecovery to enable panic recovery.
func NewUnaryServerInterceptor(o ...ServerOption) grpc.UnaryServerInterceptor {
	opts := serverOptions{
		tracer:  elasticapm.DefaultTracer,
		recover: false,
	}
	for _, o := range o {
		o(&opts)
	}
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		tx := opts.tracer.StartTransaction(info.FullMethod, "grpc")
		ctx = elasticapm.ContextWithTransaction(ctx, tx)
		defer tx.Done(-1)

		if tx.Sampled() {
			p, ok := peer.FromContext(ctx)
			if ok {
				grpcContext := map[string]interface{}{
					"peer.address": p.Addr.String(),
				}
				if p.AuthInfo != nil {
					grpcContext["auth"] = map[string]interface{}{
						"type": p.AuthInfo.AuthType(),
					}
				}
				tx.Context.SetCustom("grpc", grpcContext)
			}
		}

		defer func() {
			r := recover()
			if r != nil {
				e := opts.tracer.Recovered(r, tx)
				e.Handled = opts.recover
				e.Send()
				if opts.recover {
					err = status.Errorf(codes.Internal, "%s", r)
				} else {
					panic(r)
				}
			}
		}()

		resp, err = handler(ctx, req)
		if err == nil {
			tx.Result = codes.OK.String()
		} else {
			statusCode := codes.Unknown
			s, ok := status.FromError(err)
			if ok {
				statusCode = s.Code()
			}
			tx.Result = statusCode.String()
		}
		return resp, err
	}
}

type serverOptions struct {
	tracer  *elasticapm.Tracer
	recover bool
}

// ServerOption sets options for server-side tracing.
type ServerOption func(*serverOptions)

// WithTracer returns a ServerOption which sets t as the tracer
// to use for tracing server requests.
func WithTracer(t *elasticapm.Tracer) ServerOption {
	if t == nil {
		panic("t == nil")
	}
	return func(o *serverOptions) {
		o.tracer = t
	}
}

// WithRecovery returns a ServerOption which enables panic recovery
// in the gRPC server interceptor.
//
// The interceptor will report panics as errors to Elastic APM,
// but unless this is enabled, they will still cause the server to
// be terminated. With recovery enabled, panics will be translated
// to gRPC errors with the code gprc/codes.Internal.
func WithRecovery() ServerOption {
	return func(o *serverOptions) {
		o.recover = true
	}
}
