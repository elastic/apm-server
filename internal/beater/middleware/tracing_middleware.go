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

package middleware

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/apm-server/internal/beater/request"
)

func TracingMiddleware(tp trace.TracerProvider) Middleware {
	tracer := tp.Tracer("github.com/elastic/apm-server/internal/beater/middleware")
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			ctx, span := tracer.Start(c.Request.Context(), c.Request.Method+" "+c.Request.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			c.Request = c.Request.WithContext(ctx)
			h(c)

			span.SetAttributes(
				semconv.HTTPResponseStatusCode(c.Result.StatusCode),
				semconv.URLPath(c.Request.URL.Path),
				attribute.KeyValue{
					Key:   "http.url",
					Value: attribute.StringValue(c.Request.URL.String()),
				},
			)
		}, nil
	}
}
