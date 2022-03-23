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
	"log"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/elastic/apm-server/systemtest/benchtest"
)

func Benchmark1000Transactions(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		tracer := benchtest.NewTracer(b)
		for pb.Next() {
			for i := 0; i < 1000; i++ {
				tracer.StartTransaction("name", "type").End()
			}
			// TODO(axw) implement a transport that enables streaming
			// events in a way that we can block when the queue is full,
			// without flushing. Alternatively, make this an option in
			// TracerOptions?
			tracer.Flush(nil)
		}
	})
}

func BenchmarkOTLPTraces(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		exporter := benchtest.NewOTLPExporter(b)
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter, sdktrace.WithBlocking()),
		)
		tracer := tracerProvider.Tracer("tracer")
		for pb.Next() {
			_, span := tracer.Start(context.Background(), "name")
			span.End()
		}
		tracerProvider.ForceFlush(context.Background())
	})
}

func BenchmarkAgentGo(b *testing.B) {
	benchmarkAgent(b, `go.*.ndjson`)
}

func BenchmarkAgentNodeJS(b *testing.B) {
	benchmarkAgent(b, `nodejs.*.ndjson`)
}

func BenchmarkAgentPython(b *testing.B) {
	benchmarkAgent(b, `python.*.ndjson`)
}

func BenchmarkAgentRuby(b *testing.B) {
	benchmarkAgent(b, `ruby.*.ndjson`)
}

func benchmarkAgent(b *testing.B, expr string) {
	b.RunParallel(func(pb *testing.PB) {
		h := benchtest.NewEventHandler(b, expr)
		for pb.Next() {
			n, err := h.SendBatches(context.Background())
			if err != nil {
				b.Error("failed sending batches:", err)
			}
			if n == 0 {
				b.Errorf(
					"no events sent, ensure the '%s' matches a trace file", expr,
				)
			}
		}
	})
}

func main() {
	if err := benchtest.Run(
		Benchmark1000Transactions,
		BenchmarkOTLPTraces,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
	); err != nil {
		log.Fatal(err)
	}
}
