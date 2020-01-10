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

// +build go1.9

package apmgrpc

import (
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
)

var (
	elasticTraceparentHeader = strings.ToLower(apmhttp.ElasticTraceparentHeader)
	w3cTraceparentHeader     = strings.ToLower(apmhttp.W3CTraceparentHeader)
	tracestateHeader         = strings.ToLower(apmhttp.TracestateHeader)
)

// NewUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that
// traces gRPC requests with the given options.
//
// The interceptor will trace transactions with the "grpc" type for each
// incoming request. The transaction will be added to the context, so
// server methods can use apm.StartSpan with the provided context.
//
// By default, the interceptor will trace with apm.DefaultTracer,
// and will not recover any panics. Use WithTracer to specify an
// alternative tracer, and WithRecovery to enable panic recovery.
func NewUnaryServerInterceptor(o ...ServerOption) grpc.UnaryServerInterceptor {
	opts := serverOptions{
		tracer:         apm.DefaultTracer,
		recover:        false,
		requestIgnorer: DefaultServerRequestIgnorer(),
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
		if !opts.tracer.Active() || opts.requestIgnorer(info) {
			return handler(ctx, req)
		}
		tx, ctx := startTransaction(ctx, opts.tracer, info.FullMethod)
		defer tx.End()

		// TODO(axw) define context schema for RPC,
		// including at least the peer address.

		defer func() {
			r := recover()
			if r != nil {
				e := opts.tracer.Recovered(r)
				e.SetTransaction(tx)
				e.Context.SetFramework("grpc", grpc.Version)
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
		setTransactionResult(tx, err)
		return resp, err
	}
}

func startTransaction(ctx context.Context, tracer *apm.Tracer, name string) (*apm.Transaction, context.Context) {
	var opts apm.TransactionOptions
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		traceContext, ok := getIncomingMetadataTraceContext(md, elasticTraceparentHeader)
		if !ok {
			traceContext, _ = getIncomingMetadataTraceContext(md, w3cTraceparentHeader)
		}
		opts.TraceContext = traceContext
	}
	tx := tracer.StartTransactionOptions(name, "request", opts)
	tx.Context.SetFramework("grpc", grpc.Version)
	return tx, apm.ContextWithTransaction(ctx, tx)
}

func getIncomingMetadataTraceContext(md metadata.MD, header string) (apm.TraceContext, bool) {
	if values := md.Get(header); len(values) == 1 {
		traceContext, err := apmhttp.ParseTraceparentHeader(values[0])
		if err == nil {
			traceContext.State, _ = apmhttp.ParseTracestateHeader(md.Get(tracestateHeader)...)
			return traceContext, true
		}
	}
	return apm.TraceContext{}, false
}

func setTransactionResult(tx *apm.Transaction, err error) {
	if err == nil {
		tx.Result = codes.OK.String()
	} else {
		statusCode := codes.Unknown
		if s, ok := status.FromError(err); ok {
			statusCode = s.Code()
		}
		tx.Result = statusCode.String()
	}
}

type serverOptions struct {
	tracer         *apm.Tracer
	recover        bool
	requestIgnorer RequestIgnorerFunc
}

// ServerOption sets options for server-side tracing.
type ServerOption func(*serverOptions)

// WithTracer returns a ServerOption which sets t as the tracer
// to use for tracing server requests.
func WithTracer(t *apm.Tracer) ServerOption {
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

// RequestIgnorerFunc is the type of a function for use in
// WithServerRequestIgnorer.
type RequestIgnorerFunc func(*grpc.UnaryServerInfo) bool

// WithServerRequestIgnorer returns a ServerOption which sets r as the
// function to use to determine whether or not a server request should
// be ignored. If r is nil, all requests will be reported.
func WithServerRequestIgnorer(r RequestIgnorerFunc) ServerOption {
	if r == nil {
		r = IgnoreNone
	}
	return func(o *serverOptions) {
		o.requestIgnorer = r
	}
}
