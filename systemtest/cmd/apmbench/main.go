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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/systemtest/benchtest"
)

func Benchmark1000Transactions(b *testing.B, l *rate.Limiter) {
	b.RunParallel(func(pb *testing.PB) {
		tracer := benchtest.NewTracer(b)
		for pb.Next() {
			for i := 0; i < 1000; i++ {
				if err := l.Wait(context.Background()); err != nil {
					b.Fatal(err)
				}
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

func BenchmarkOTLPTraces(b *testing.B, l *rate.Limiter) {
	switch strings.ToLower(os.Getenv("RUN_OTLPTRACES")) {
	case "1", "true":
	default:
		b.Skip("Disabled until we've addressed https://github.com/elastic/apm-server/issues/9242, export RUN_OTLPTRACES=1 to run")
	}

	b.RunParallel(func(pb *testing.PB) {
		exporter := benchtest.NewOTLPExporter(b)
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter, sdktrace.WithBlocking()),
		)
		tracer := tracerProvider.Tracer("tracer")
		for pb.Next() {
			if err := l.Wait(context.Background()); err != nil {
				b.Fatal(err)
			}
			_, span := tracer.Start(context.Background(), "name")
			span.End()
		}
		tracerProvider.ForceFlush(context.Background())
	})
}

// BenchmarkAgentAll matches all the agent event files, allowing the benchmark
// to contain a wider mix of events compared to the other BenchmarkAgent<Name>
// benchmarks. The objective is to measure how the APM Server performs when it
// receives events from multiple agents.
// Even though files are loaded alphabetically and the events sent sequentially
// there is inherent randomness in the order the events are sent to APM Sever.
func BenchmarkAgentAll(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `*.ndjson`)
}

func BenchmarkAgentGo(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `go*.ndjson`)
}

func BenchmarkAgentNodeJS(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `nodejs*.ndjson`)
}

func BenchmarkAgentPython(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `python*.ndjson`)
}

func BenchmarkAgentRuby(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `ruby*.ndjson`)
}

func benchmarkAgent(b *testing.B, l *rate.Limiter, expr string) {
	h := benchtest.NewEventHandler(b, expr, l)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.SendBatches(context.Background())
		}
	})
}

func Benchmark10000AggregationGroups(b *testing.B, l *rate.Limiter) {
	// Benchmark memory usage on aggregating high cardinality data.
	b.RunParallel(func(pb *testing.PB) {
		tracer := benchtest.NewTracer(b)
		for pb.Next() {
			for i := 0; i < 10000; i++ {
				if err := l.Wait(context.Background()); err != nil {
					b.Fatal(err)
				}
				tracer.StartTransaction(fmt.Sprintf("name%d", i), fmt.Sprintf("type%d", i)).End()
			}
			tracer.Flush(nil)
		}
	})
}

func main() {
	flag.Parse()
	if err := benchtest.Run(
		Benchmark1000Transactions,
		BenchmarkOTLPTraces,
		BenchmarkAgentAll,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
		Benchmark10000AggregationGroups,
	); err != nil {
		log.Fatal(err)
	}
}
