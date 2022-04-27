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
	"flag"
	"fmt"
	"net"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

var (
	endpoint = flag.String(
		"endpoint",
		"http://localhost:8200",
		"target URL to which sendotlp will send spans and metrics",
	)

	secretToken = flag.String(
		"secret-token",
		"",
		"Elastic APM secret token",
	)

	logLevel = zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)
)

func main() {
	flag.Parse()
	zapcfg := zap.NewProductionConfig()
	zapcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.Encoding = "console"
	logger, err := zapcfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	grpclog.SetLogger(zapgrpc.NewLogger(logger, zapgrpc.WithDebug()))

	if err := Main(context.Background(), logger.Sugar()); err != nil {
		logger.Fatal("error sending data", zap.Error(err))
	}
}

func Main(ctx context.Context, logger *zap.SugaredLogger) error {
	var insecure bool
	endpointURL, err := url.Parse(*endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}
	switch endpointURL.Scheme {
	case "http":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "80")
		}
		insecure = true
	case "https":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "443")
		}
	default:
		return fmt.Errorf(
			"endpoint must be prefixed with http:// or https://",
		)
	}
	logger.Infof("sending OTLP data to %s", endpointURL.String())
	grpcEndpoint := endpointURL.Host

	var grpcDialOptions []grpc.DialOption
	if insecure {
		// If http:// is specified, then use insecure (plaintext).
		grpcDialOptions = append(grpcDialOptions, grpc.WithInsecure())
	} else {
		grpcDialOptions = append(grpcDialOptions, grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		))
	}
	grpcConn, err := grpc.DialContext(ctx, grpcEndpoint, grpcDialOptions...)
	if err != nil {
		return err
	}

	headers := make(map[string]string)
	if *secretToken != "" {
		headers["Authorization"] = "Bearer " + *secretToken
	}

	otlpTraceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(grpcConn),
		otlptracegrpc.WithHeaders(headers),
	)
	if err != nil {
		return err
	}
	var tracerProviderOptions []sdktrace.TracerProviderOption
	tracerProviderOptions = append(tracerProviderOptions,
		sdktrace.WithSyncer(loggingExporter{logger.Desugar()}),
		sdktrace.WithSyncer(otlpTraceExporter),
	)
	tracerProvider := sdktrace.NewTracerProvider(tracerProviderOptions...)

	otlpMetricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(grpcConn),
		otlpmetricgrpc.WithHeaders(headers),
	)
	if err != nil {
		return err
	}
	metricExporter := chainMetricExporter{loggingExporter{logger.Desugar()}, otlpMetricExporter}
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

func (chainMetricExporter) TemporalityFor(desc *sdkapi.Descriptor, kind aggregation.Kind) aggregation.Temporality {
	return aggregation.StatelessTemporalitySelector().TemporalityFor(desc, kind)
}

type loggingExporter struct {
	logger *zap.Logger
}

func (loggingExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (loggingExporter) MarshalLog() interface{} {
	return "loggingExporter"
}

func (e loggingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, span := range tracetest.SpanStubsFromReadOnlySpans(spans) {
		e.logger.Info("exporting span", zap.Any("span", span))
	}
	return nil
}

func (e loggingExporter) Export(ctx context.Context, resource *resource.Resource, r export.InstrumentationLibraryReader) error {
	return r.ForEach(func(_ instrumentation.Library, mr export.Reader) error {
		return mr.ForEach(e, func(record export.Record) error {
			fields := []zap.Field{zap.Namespace("metric")}

			desc := record.Descriptor()
			fields = append(fields, zap.String("name", desc.Name()))
			fields = append(fields, zap.String("kind", desc.InstrumentKind().String()))
			if unit := desc.Unit(); unit != "" {
				fields = append(fields, zap.String("unit", string(unit)))
			}
			fields = append(fields, zap.Time("start_time", record.StartTime()))
			fields = append(fields, zap.Time("end_time", record.EndTime()))

			agg := record.Aggregation()
			if agg, ok := agg.(aggregation.Histogram); ok {
				buckets, err := agg.Histogram()
				if err != nil {
					return err
				}
				fields = append(fields, zap.Float64s("boundaries", buckets.Boundaries))
				fields = append(fields, zap.Uint64s("counts", buckets.Counts))
			}
			if agg, ok := agg.(aggregation.Sum); ok {
				sum, err := agg.Sum()
				if err != nil {
					return err
				}
				fields = append(fields, zap.Float64("sum", sum.CoerceToFloat64(desc.NumberKind())))
			}
			if agg, ok := agg.(aggregation.Count); ok {
				count, err := agg.Count()
				if err != nil {
					return err
				}
				fields = append(fields, zap.Uint64("count", count))
			}

			e.logger.Info("exporting metric", fields...)
			return nil
		})
	})
}

func (loggingExporter) TemporalityFor(desc *sdkapi.Descriptor, kind aggregation.Kind) aggregation.Temporality {
	return aggregation.StatelessTemporalitySelector().TemporalityFor(desc, kind)
}
