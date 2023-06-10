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
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
)

var (
	endpoint = flag.String(
		"endpoint",
		getenvDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:8200"),
		"target URL to which sendotlp will send spans and metrics ($OTEL_EXPORTER_OTLP_ENDPOINT)",
	)

	secretToken = flag.String(
		"secret-token",
		"",
		"Elastic APM secret token. Note: setting this overrides $OTEL_EXPORTER_OTLP_HEADERS",
	)

	logLevel = zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of: DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)

	protocol = flag.String(
		"protocol",
		getenvDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		"set transport protocol to one of: grpc (default), http/protobuf",
	)

	insecure = flag.Bool(
		"insecure",
		false,
		"skip the server's TLS certificate verification",
	)
)

func getenvDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return defaultVal
}

func main() {
	flag.Parse()
	zapcfg := zap.NewProductionConfig()
	zapcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.Encoding = "console"
	zapcfg.Level = zap.NewAtomicLevelAt(*logLevel)
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
	endpointURL, err := url.Parse(*endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}
	switch endpointURL.Scheme {
	case "http":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "80")
		}
	case "https":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "443")
		}
	default:
		return fmt.Errorf("endpoint must be prefixed with http:// or https://")
	}

	otlpExporters, err := newOTLPExporters(ctx, endpointURL)
	if err != nil {
		return err
	}
	defer otlpExporters.cleanup(ctx)

	logger.Infof("sending OTLP data to %s (%s)", endpointURL.String(), *protocol)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(loggingExporter{logger.Desugar()}),
		sdktrace.WithSyncer(otlpExporters.trace),
	)
	metricExporter := chainMetricExporter{loggingExporter{logger.Desugar()}, otlpExporters.metric}

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

	logExporter := chainLogExporter{loggingLogExporter{logger.Desugar()}, otlpExporters.log}
	// Generate some data. Metrics are sent when the controller is stopped, traces and logs
	// are sent immediately.
	//
	if err := generateMetrics(ctx, metricsController.Meter("sendotlp")); err != nil {
		return err
	}
	if err := metricsController.Stop(ctx); err != nil {
		return err
	}
	if err := generateSpans(ctx, tracerProvider.Tracer("sendotlp")); err != nil {
		return err
	}
	if err := generateLogs(ctx, logExporter); err != nil {
		return err
	}

	// Shutdown, flushing all data to the server.
	if err := tracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	return otlpExporters.cleanup(ctx)
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
	counter, err := meter.SyncFloat64().Counter("float64_counter")
	if err != nil {
		return err
	}
	counter.Add(ctx, 1)

	hist, err := meter.SyncInt64().Histogram("int64_histogram")
	if err != nil {
		return err
	}
	hist.Record(ctx, 1)
	hist.Record(ctx, 10)
	hist.Record(ctx, 10)
	hist.Record(ctx, 100)
	hist.Record(ctx, 100)
	hist.Record(ctx, 100)

	return nil
}

func generateLogs(ctx context.Context, logger otlplogExporter) error {
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	attribs := rl.Resource().Attributes()
	attribs.Insert(string(semconv.ServiceNameKey),
		pcommon.NewValueString(getenvDefault("OTEL_SERVICE_NAME", "unknown_service")))
	sl := rl.ScopeLogs().AppendEmpty().LogRecords()
	record := sl.AppendEmpty()
	record.Body().SetStringVal("test record")
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	record.SetSeverityNumber(plog.SeverityNumberFATAL)
	record.SetSeverityText("fatal")

	return logger.Export(ctx, logs)
}

type otlpExporters struct {
	cleanup func(context.Context) error
	trace   *otlptrace.Exporter
	metric  *otlpmetric.Exporter
	log     otlplogExporter
}

func newOTLPExporters(ctx context.Context, endpointURL *url.URL) (*otlpExporters, error) {
	switch *protocol {
	case "grpc":
		return newOTLPGRPCExporters(ctx, endpointURL)
	case "http/protobuf":
		return newOTLPHTTPExporters(ctx, endpointURL)
	default:
		return nil, fmt.Errorf("invalid protocol %q", *protocol)
	}
}

func newOTLPGRPCExporters(ctx context.Context, endpointURL *url.URL) (*otlpExporters, error) {
	var transportCredentials credentials.TransportCredentials

	switch endpointURL.Scheme {
	case "http":
		// If http:// is specified, then use insecure (plaintext).
		transportCredentials = grpcinsecure.NewCredentials()
	case "https":
		transportCredentials = credentials.NewTLS(&tls.Config{InsecureSkipVerify: *insecure})
	}

	grpcConn, err := grpc.DialContext(ctx, endpointURL.Host, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		return nil, err
	}
	cleanup := func(context.Context) error {
		return grpcConn.Close()
	}

	traceOptions := []otlptracegrpc.Option{otlptracegrpc.WithGRPCConn(grpcConn)}
	metricOptions := []otlpmetricgrpc.Option{otlpmetricgrpc.WithGRPCConn(grpcConn)}
	var logHeaders map[string]string

	if *secretToken != "" {
		// If -secret-token is specified then we set headers explicitly,
		// overriding anything set in $OTEL_EXPORTER_OTLP_HEADERS.
		headers := map[string]string{"Authorization": "Bearer " + *secretToken}
		traceOptions = append(traceOptions, otlptracegrpc.WithHeaders(headers))
		metricOptions = append(metricOptions, otlpmetricgrpc.WithHeaders(headers))
		logHeaders = headers
	}

	otlpTraceExporter, err := otlptracegrpc.New(ctx, traceOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpTraceExporter.Shutdown, cleanup)

	otlpMetricExporter, err := otlpmetricgrpc.New(ctx, metricOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpMetricExporter.Shutdown, cleanup)

	return &otlpExporters{
		cleanup: cleanup,
		trace:   otlpTraceExporter,
		metric:  otlpMetricExporter,
		log: &otlploggrpcExporter{
			client:  plogotlp.NewClient(grpcConn),
			headers: logHeaders,
		},
	}, nil
}

func newOTLPHTTPExporters(ctx context.Context, endpointURL *url.URL) (*otlpExporters, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: *insecure}
	traceOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpointURL.Host),
		otlptracehttp.WithTLSClientConfig(tlsConfig),
	}
	metricOptions := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpointURL.Host),
		otlpmetrichttp.WithTLSClientConfig(tlsConfig),
	}
	if endpointURL.Scheme == "http" {
		traceOptions = append(traceOptions, otlptracehttp.WithInsecure())
		metricOptions = append(metricOptions, otlpmetrichttp.WithInsecure())
	}
	if *secretToken != "" {
		// If -secret-token is specified then we set headers explicitly,
		// overriding anything set in $OTEL_EXPORTER_OTLP_HEADERS.
		headers := map[string]string{"Authorization": "Bearer " + *secretToken}
		traceOptions = append(traceOptions, otlptracehttp.WithHeaders(headers))
		metricOptions = append(metricOptions, otlpmetrichttp.WithHeaders(headers))
	}

	cleanup := func(context.Context) error { return nil }

	otlpTraceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpTraceExporter.Shutdown, cleanup)

	otlpMetricExporter, err := otlpmetrichttp.New(ctx, metricOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpMetricExporter.Shutdown, cleanup)

	return &otlpExporters{
		cleanup: cleanup,
		trace:   otlpTraceExporter,
		metric:  otlpMetricExporter,
		log:     &otlploghttpExporter{},
	}, nil
}

func combineCleanup(a, b func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := a(ctx); err != nil {
			return err
		}
		return b(ctx)
	}
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

type otlplogExporter interface {
	Export(ctx context.Context, logs plog.Logs) error
}

// otlploggrpcExporter is a simple synchronous log exporter using GRPC
type otlploggrpcExporter struct {
	client  plogotlp.Client
	headers map[string]string
}

func (e *otlploggrpcExporter) Export(ctx context.Context, logs plog.Logs) error {
	req := plogotlp.NewRequestFromLogs(logs)
	md := metadata.New(e.headers)
	ctx = metadata.NewOutgoingContext(ctx, md)

	_, err := e.client.Export(ctx, req)
	if err != nil {
		return err
	}

	// TODO: parse response for error
	return nil
}

// otlploghttpExporter is a simple synchronous log exporter using protobuf over HTTP
type otlploghttpExporter struct {
}

func (e *otlploghttpExporter) Export(ctx context.Context, logs plog.Logs) error {
	// TODO: implement
	return nil
}

type chainLogExporter []otlplogExporter

func (c chainLogExporter) Export(ctx context.Context, logs plog.Logs) error {
	for _, exporter := range c {
		if err := exporter.Export(ctx, logs); err != nil {
			return err
		}
	}
	return nil
}

type loggingLogExporter struct {
	logger *zap.Logger
}

func (e loggingLogExporter) Export(ctx context.Context, logs plog.Logs) error {
	e.logger.Info("exporting logs", zap.Int("count", logs.LogRecordCount()))
	return nil
}
