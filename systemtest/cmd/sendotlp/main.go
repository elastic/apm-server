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
	"errors"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	if err := Main(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func Main(ctx context.Context) error {
	const otelEndpointEnv = "OTEL_EXPORTER_OTLP_ENDPOINT"
	endpoint := os.Getenv(otelEndpointEnv)
	if endpoint == "" {
		endpoint = "http://localhost:8200"
		os.Setenv(otelEndpointEnv, endpoint)
	}
	log.Printf("Sending OTLP data to %s ($%s)", endpoint, otelEndpointEnv)

	var otlpTraceExporterOptions []otlptracegrpc.Option
	otlpTraceExporterOptions = append(otlpTraceExporterOptions, otlptracegrpc.WithInsecure())
	otlpTraceExporter, err := otlptracegrpc.New(ctx, otlpTraceExporterOptions...)
	if err != nil {
		return err
	}
	stdoutTraceExporter, err := stdouttrace.New()
	if err != nil {
		return err
	}
	var tracerProviderOptions []sdktrace.TracerProviderOption
	tracerProviderOptions = append(tracerProviderOptions,
		sdktrace.WithSyncer(stdoutTraceExporter),
		sdktrace.WithSyncer(otlpTraceExporter),
	)
	tracerProvider := sdktrace.NewTracerProvider(tracerProviderOptions...)

	var otlpMetricExporterOptions []otlpmetricgrpc.Option
	otlpMetricExporterOptions = append(otlpMetricExporterOptions, otlpmetricgrpc.WithInsecure())
	otlpMetricExporter, err := otlpmetricgrpc.New(ctx, otlpMetricExporterOptions...)
	if err != nil {
		return err
	}
	stdoutMetricExporter, err := stdoutmetric.New()
	if err != nil {
		return err
	}
	metricExporter := chainMetricExporter{stdoutMetricExporter, otlpMetricExporter}
	metricsController := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries([]float64{1, 10, 100, 1000, 10000}),
			),
			aggregation.CumulativeTemporalitySelector(),
		),
		controller.WithExporter(metricExporter),
		controller.WithCollectPeriod(time.Hour),
	)
	if err := metricsController.Start(ctx); err != nil {
		return err
	}

	// Generate some data. Metrics are sent when the controller is stopped, traces
	// are sent immediately.
	//
	// TODO(axw) generate logs when opentelemetry-go has support.
	if err := generateMetrics(ctx, metricsController.Meter("sendotlp")); err != nil {
		return err
	}
	if err := metricsController.Stop(ctx); err != nil {
		return err
	}
	if err := generateSpans(ctx, tracerProvider.Tracer("sendotlp")); err != nil {
		return err
	}

	// Shutdown, flushing all data to the server.
	if otlpMetricExporter.Shutdown(ctx); err != nil {
		return err
	}
	if err := tracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	if err := otlpTraceExporter.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func generateSpans(ctx context.Context, tracer trace.Tracer) error {
	ctx, parent := tracer.Start(ctx, "parent")
	defer parent.End()

	_, child1 := tracer.Start(ctx, "child1")
	time.Sleep(10 * time.Millisecond)
	child1.AddEvent("an arbitrary event")
	child1.End()

	_, child2 := tracer.Start(ctx, "child2")
	time.Sleep(10 * time.Millisecond)
	child2.RecordError(errors.New("an exception occurred"))
	child2.End()

	return nil
}

func generateMetrics(ctx context.Context, meter metric.Meter) error {
	metric.Must(meter).NewFloat64Counter("float64_counter").Add(ctx, 1)

	hist := metric.Must(meter).NewInt64Histogram("int64_histogram")
	hist.Record(ctx, 1)
	hist.Record(ctx, 10)
	hist.Record(ctx, 10)
	hist.Record(ctx, 100)
	hist.Record(ctx, 100)
	hist.Record(ctx, 100)

	return nil
}

type chainMetricExporter []export.Exporter

func (c chainMetricExporter) Export(ctx context.Context, resource *resource.Resource, r export.InstrumentationLibraryReader) error {
	for _, exporter := range c {
		if err := exporter.Export(ctx, resource, r); err != nil {
			return err
		}
	}
	return nil
}

func (c chainMetricExporter) TemporalityFor(descriptor *sdkapi.Descriptor, aggregationKind aggregation.Kind) aggregation.Temporality {
	panic("unexpected call")
}
