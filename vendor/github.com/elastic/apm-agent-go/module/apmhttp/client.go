package apmhttp

import (
	"net/http"

	"github.com/elastic/apm-agent-go"
)

// WrapClient returns a new *http.Client with all fields copied
// across, and the Transport field wrapped with WrapRoundTripper
// such that client requests are reported as spans to Elastic APM
// if their context contains a sampled transaction.
//
// If c is nil, then http.DefaultClient is wrapped.
func WrapClient(c *http.Client, o ...ClientOption) *http.Client {
	if c == nil {
		c = http.DefaultClient
	}
	copied := *c
	copied.Transport = WrapRoundTripper(copied.Transport, o...)
	return &copied
}

// WrapRoundTripper returns an http.RoundTripper wrapping r, reporting each
// request as a span to Elastic APM, if the request's context contains a
// sampled transaction.
//
// If r is nil, then http.DefaultTransport is wrapped.
func WrapRoundTripper(r http.RoundTripper, o ...ClientOption) http.RoundTripper {
	if r == nil {
		r = http.DefaultTransport
	}
	rt := &roundTripper{
		r:              r,
		requestName:    ClientRequestName,
		requestIgnorer: ignoreNone,
	}
	for _, o := range o {
		o(rt)
	}
	return rt
}

type roundTripper struct {
	r              http.RoundTripper
	requestName    RequestNameFunc
	requestIgnorer RequestIgnorerFunc
}

// RoundTrip delegates to r.r, emitting a span if req's context
// contains a transaction.
func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.requestIgnorer(req) {
		return r.r.RoundTrip(req)
	}
	ctx := req.Context()
	tx := elasticapm.TransactionFromContext(ctx)
	if tx == nil || !tx.Sampled() {
		return r.r.RoundTrip(req)
	}

	name := r.requestName(req)
	spanType := "ext.http"
	span := tx.StartSpan(name, spanType, elasticapm.SpanFromContext(ctx))
	defer span.End()

	ctx = elasticapm.ContextWithSpan(ctx, span)
	req = RequestWithContext(ctx, req)
	return r.r.RoundTrip(req)
}

// ClientOption sets options for tracing client requests.
type ClientOption func(*roundTripper)
