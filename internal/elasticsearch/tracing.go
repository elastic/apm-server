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

package elasticsearch

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func WrapRoundTripper(r http.RoundTripper, tp trace.TracerProvider) http.RoundTripper {
	if tp == nil {
		tp = noop.NewTracerProvider()
	}
	rt := &roundTripper{
		r:      r,
		tracer: tp.Tracer("github.com/elastic/apm-server/internal/elasticsearch"),
	}
	return rt
}

type roundTripper struct {
	r      http.RoundTripper
	tracer trace.Tracer
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	name := requestName(req)
	ctx, span := r.tracer.Start(req.Context(), name, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	req = req.WithContext(ctx)

	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header))

	span.SetAttributes(
		attribute.String("db.type", "elasticsearch"),
		attribute.String("db.system", "elasticsearch"),
	)

	resp, err := r.r.RoundTrip(req)

	if resp != nil {
		span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

		if clusterName := resp.Header.Get("X-Found-Handling-Cluster"); clusterName != "" {
			span.SetAttributes(attribute.String("db.instance", clusterName))
		}
	}

	return resp, err
}

// CloseIdleConnections calls r.r.CloseIdleConnections if the method exists.
func (r *roundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := r.r.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

func requestName(req *http.Request) string {
	const prefix = "Elasticsearch:"
	path := strings.TrimLeft(req.URL.Path, "/")

	var b strings.Builder
	b.Grow(len(prefix) + 1 + len(req.Method) + 1 + len(path))
	b.WriteString(prefix)
	b.WriteRune(' ')
	b.WriteString(req.Method)
	b.WriteRune(' ')
	b.WriteString(path)
	return b.String()
}
