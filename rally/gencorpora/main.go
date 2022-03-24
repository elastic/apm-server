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

package main

import (
	"context"
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	GenerateCorpus("http_and_sql", func(newTracer newTracerFunc) {
		backendTracer := newTracer("backend")
		for i := 0; i < 10000; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			ctx, httpSpan := backendTracer.Start(context.Background(), "GET /customers/:id",
				trace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest("", "/customers/:id", req)...),
				trace.WithAttributes(semconv.HTTPAttributesFromHTTPStatusCode(http.StatusOK)...),
			)
			ctx, dbSpan := backendTracer.Start(ctx, "SELECT FROM customers",
				trace.WithAttributes(
					semconv.DBNameKey.String("customerdb"),
					semconv.DBStatementKey.String("SELECT * FROM customers WHERE id=? LIMIT 1"),
					semconv.DBSystemMSSQL,
				),
			)
			dbSpan.End()
			httpSpan.End()
		}
	})
}
