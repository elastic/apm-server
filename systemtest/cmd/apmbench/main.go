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
	"embed"
	"flag"
	"fmt"
	"log"
	"runtime/debug"
	"testing"
	"time"

	"go.elastic.co/apm/v2"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/systemtest/benchtest"
	"github.com/elastic/go-sysinfo"
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
	benchmarkAgent(b, l, `apm-*.ndjson`)
}

func BenchmarkAgentGo(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-go*.ndjson`)
}

func BenchmarkAgentNodeJS(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-nodejs*.ndjson`)
}

func BenchmarkAgentPython(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-python*.ndjson`)
}

func BenchmarkAgentRuby(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-ruby*.ndjson`)
}

func benchmarkAgent(b *testing.B, l *rate.Limiter, expr string) {
	h := benchtest.NewEventHandler(b, expr, l)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.SendBatches(context.Background())
		}
	})
}

// events contains custom events that are not in apm-perf.
//
//go:embed events/*.ndjson
var events embed.FS

func BenchmarkTracesAgentAll(b *testing.B, l *rate.Limiter) {
	benchmarkTracesAgent(b, l, `apm-*.ndjson`)
}

func BenchmarkTracesAgentGo(b *testing.B, l *rate.Limiter) {
	benchmarkTracesAgent(b, l, `apm-go*.ndjson`)
}

func BenchmarkTracesAgentNodeJS(b *testing.B, l *rate.Limiter) {
	benchmarkTracesAgent(b, l, `apm-nodejs*.ndjson`)
}

func BenchmarkTracesAgentPython(b *testing.B, l *rate.Limiter) {
	benchmarkTracesAgent(b, l, `apm-python*.ndjson`)
}

func BenchmarkTracesAgentRuby(b *testing.B, l *rate.Limiter) {
	benchmarkTracesAgent(b, l, `apm-ruby*.ndjson`)
}

// benchmarkTracesAgent benchmarks with traces only. Useful to benchmark TBS.
func benchmarkTracesAgent(b *testing.B, l *rate.Limiter, expr string) {
	h := benchtest.NewFSEventHandler(b, expr, l, events)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.SendBatches(context.Background())
		}
	})
}

func Benchmark10000AggregationGroups(b *testing.B, l *rate.Limiter) {
	// Benchmark memory usage on aggregating high cardinality data.
	// This should generate a lot of groups for service transaction metrics,
	// transaction metrics, and service destination metrics.
	//
	// Using b.N instead of b.RunParallel since this benchmark is about memory
	// usage.
	//
	// If rate limiter is used, it is possible that part of the 10k
	// transactions will not fit into the same 1m aggregation period, and this
	// will cause a lower observed memory usage.
	for n := 0; n < b.N; n++ {
		tracer := benchtest.NewTracer(b)
		for i := 0; i < 10000; i++ {
			if err := l.Wait(context.Background()); err != nil {
				b.Fatal(err)
			}
			tx := tracer.StartTransaction(fmt.Sprintf("name%d", i), fmt.Sprintf("type%d", i))
			span := tx.StartSpanOptions(fmt.Sprintf("name%d", i), fmt.Sprintf("type%d", i), apm.SpanOptions{})
			span.Context.SetDestinationService(apm.DestinationServiceSpanContext{
				Name:     fmt.Sprintf("name%d", i),
				Resource: fmt.Sprintf("resource%d", i),
			})
			span.Duration = time.Second
			span.End()
			tx.End()
		}
		tracer.Flush(nil)
	}
}

func main() {
	flag.Parse()
	bytes, err := sysMemory()
	if err != nil {
		log.Fatal(err)
	}
	debug.SetMemoryLimit(int64(float64(bytes) * 0.9))
	if err := benchtest.Run(
		Benchmark1000Transactions,
		BenchmarkOTLPTraces,
		BenchmarkAgentAll,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
		Benchmark10000AggregationGroups,
		BenchmarkTracesAgentAll,
		BenchmarkTracesAgentGo,
		BenchmarkTracesAgentNodeJS,
		BenchmarkTracesAgentPython,
		BenchmarkTracesAgentRuby,
	); err != nil {
		log.Fatal(err)
	}
}

func sysMemory() (uint64, error) {
	host, err := sysinfo.Host()
	if err != nil {
		return 0, err
	}
	mem, err := host.Memory()
	if err != nil {
		return 0, err
	}
	return mem.Total, nil
}
