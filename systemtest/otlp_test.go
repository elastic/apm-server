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

package systemtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/model/otlpgrpc"
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

var otelErrors = make(chan error, 1)

func init() {
	// otel.SetErrorHandler can only be called once per process.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		if err == nil {
			return
		}
		select {
		case otelErrors <- err:
		default:
		}
	}))
}

func TestOTLPGRPCTraces(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource, err := resource.Merge(resource.Default(), sdkresource.NewSchemaless(
		attribute.StringSlice("resource_attribute_array", []string{"a", "b"}),
		attribute.Bool("resource_attribute_bool", true),
		attribute.BoolSlice("resource_attribute_bool_array", []bool{true, false}),
		attribute.Float64("resource_attribute_float", 123456.789),
		attribute.Float64Slice("resource_attribute_float_array", []float64{123456.789, 987654321.123456789}),
		attribute.Int64("resource_attribute_int", 123456),
	))
	require.NoError(t, err)

	err = withOTLPTracer(newOTLPTracerProvider(newOTLPTraceExporter(t, srv), sdktrace.WithResource(resource)), func(tracer trace.Tracer) {
		startTime := time.Unix(123, 456)
		endTime := startTime.Add(time.Second)
		_, span := tracer.Start(ctx, "operation_name", trace.WithTimestamp(startTime), trace.WithAttributes(
			attribute.StringSlice("span_attribute_array", []string{"a", "b", "c"}),
		))
		span.AddEvent("a_span_event", trace.WithTimestamp(startTime.Add(time.Millisecond)))
		span.End(trace.WithTimestamp(endTime))
	})
	require.NoError(t, err)

	indices := "traces-apm*,logs-apm*"
	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, indices, estest.BoolQuery{
		Should: []interface{}{
			estest.TermQuery{Field: "processor.event", Value: "transaction"},
			estest.TermQuery{Field: "processor.event", Value: "log"},
		},
		MinimumShouldMatch: 1,
	})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestOTLPGRPCMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	err := srv.Start()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	aggregator := simple.NewWithHistogramDistribution(histogram.WithExplicitBoundaries([]float64{1, 100, 1000, 10000}))
	err = sendOTLPMetrics(t, ctx, srv, aggregator, func(meter metric.MeterMust) {
		float64Counter := meter.NewFloat64Counter("float64_counter")
		float64Counter.Add(context.Background(), 1)

		int64Histogram := meter.NewInt64Histogram("int64_histogram")
		int64Histogram.Record(context.Background(), 1)
		int64Histogram.Record(context.Background(), 123)
		int64Histogram.Record(context.Background(), 1024)
		int64Histogram.Record(context.Background(), 20000)
	})
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectDocs(t, "metrics-apm.app.*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "metric"},
	}})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits, "@timestamp")

	// Make sure we report monitoring for the metrics consumer. Metric values are unit tested.
	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.otlp.grpc.metrics.consumer").Exists())
}

func TestOTLPGRPCLogs(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := grpc.Dial(serverAddr(srv), grpc.WithInsecure(), grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	logsClient := otlpgrpc.NewLogsClient(conn)

	logs := newLogs("a log message")
	_, err = logsClient.Export(ctx, logs)
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectDocs(t, "logs-apm*", estest.TermQuery{
		Field: "processor.event", Value: "log",
	})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestOTLPGRPCAuth(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.AgentAuth.SecretToken = "abc123"
	err := srv.Start()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(newOTLPTraceExporter(t, srv)))
	assert.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(newOTLPTraceExporter(t, srv, otlptracegrpc.WithHeaders(map[string]string{
		"Authorization": "Bearer abc123",
	}))))
	require.NoError(t, err)
	systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	}})
}

func TestOTLPClientIP(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporter := newOTLPTraceExporter(t, srv)
	err := sendOTLPTrace(ctx, newOTLPTracerProvider(exporter))
	assert.NoError(t, err)

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(
		sdkresource.NewSchemaless(attribute.String("service.name", "service1")),
	)))
	require.NoError(t, err)

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(
		sdkresource.NewSchemaless(
			attribute.String("service.name", "service2"),
			attribute.String("telemetry.sdk.name", "iOS"),
			attribute.String("telemetry.sdk.language", "swift"),
		),
	)))
	require.NoError(t, err)

	// Non-iOS agent documents should have no client.ip field set.
	result := systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", estest.TermQuery{
		Field: "service.name", Value: "service1",
	})
	assert.False(t, gjson.GetBytes(result.Hits.Hits[0].RawSource, "client.ip").Exists())

	// iOS agent documents should have a client.ip field set.
	result = systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", estest.TermQuery{
		Field: "service.name", Value: "service2",
	})
	assert.True(t, gjson.GetBytes(result.Hits.Hits[0].RawSource, "client.ip").Exists())
}

func TestOTLPAnonymous(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.AgentAuth.SecretToken = "abc123" // enable auth & rate limiting
	srv.Config.AgentAuth.Anonymous = &apmservertest.AnonymousAuthConfig{
		Enabled:      true,
		AllowAgent:   []string{"iOS/swift"},
		AllowService: []string{"allowed_service"},
	}
	err := srv.Start()
	require.NoError(t, err)

	sendEvent := func(telemetrySDKName, telemetrySDKLanguage, serviceName string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var attributes []attribute.KeyValue
		if serviceName != "" {
			attributes = append(attributes, attribute.String("service.name", serviceName))
		}
		if telemetrySDKName != "" {
			attributes = append(attributes, attribute.String("telemetry.sdk.name", telemetrySDKName))
		}
		if telemetrySDKLanguage != "" {
			attributes = append(attributes, attribute.String("telemetry.sdk.language", telemetrySDKLanguage))
		}
		exporter := newOTLPTraceExporter(t, srv)
		resource := sdkresource.NewSchemaless(attributes...)
		return sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(resource)))
	}

	err = sendEvent("iOS", "swift", "allowed_service")
	assert.NoError(t, err)

	err = sendEvent("open-telemetry", "go", "allowed_service")
	assert.Error(t, err)
	errStatus, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, errStatus.Code())
	assert.Equal(t, `unauthorized: anonymous access not permitted for agent "open-telemetry/go"`, errStatus.Message())

	err = sendEvent("iOS", "swift", "unallowed_service")
	assert.Error(t, err)
	errStatus, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, errStatus.Code())
	assert.Equal(t, `unauthorized: anonymous access not permitted for service "unallowed_service"`, errStatus.Message())

	// If the client does not send telemetry.sdk.*, we default agent name "otlp".
	// This means it is not possible to bypass the allowed agents list.
	err = sendEvent("", "", "allowed_service")
	assert.Error(t, err)
	errStatus, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, errStatus.Code())
	assert.Equal(t, `unauthorized: anonymous access not permitted for agent "otlp"`, errStatus.Message())

	// If the client does not send a service name, we default to "unknown".
	// This means it is not possible to bypass the allowed services list.
	err = sendEvent("iOS", "swift", "")
	assert.Error(t, err)
	errStatus, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, errStatus.Code())
	assert.Equal(t, `unauthorized: anonymous access not permitted for service "unknown"`, errStatus.Message())
}

func TestOTLPRateLimit(t *testing.T) {
	// The configured rate limit.
	const eventRateLimit = 10

	// The actual rate limit: a 3x "burst multiplier" is applied,
	// and each gRPC method call is counted towards the event rate
	// limit as well.
	const sendEventLimit = (3 * eventRateLimit) / 2

	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.AgentAuth.SecretToken = "abc123" // enable auth & rate limiting
	srv.Config.AgentAuth.Anonymous = &apmservertest.AnonymousAuthConfig{
		Enabled:    true,
		AllowAgent: []string{"iOS/swift"},
		RateLimit: &apmservertest.RateLimitConfig{
			IPLimit:    2,
			EventLimit: eventRateLimit,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	sendEvent := func(ctx context.Context, ip string) error {
		exporter := newOTLPTraceExporter(t, srv,
			otlptracegrpc.WithHeaders(map[string]string{"x-real-ip": ip}),
			otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{Enabled: false}),
		)
		resource := sdkresource.NewSchemaless(
			attribute.String("service.name", "service2"),
			attribute.String("telemetry.sdk.name", "iOS"),
			attribute.String("telemetry.sdk.language", "swift"),
		)
		return sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(resource)))
	}

	// Check that for the configured IP limit (2), we can handle 3*event_limit without being rate limited.
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < sendEventLimit; i++ {
		g.Go(func() error { return sendEvent(ctx, "10.11.12.13") })
		g.Go(func() error { return sendEvent(ctx, "10.11.12.14") })
	}
	err = g.Wait()
	assert.NoError(t, err)

	// The rate limiter cache only has space for 2 IPs, so the 3rd one reuses an existing
	// limiter which should have already been exhausted. However, the rate limiter may be
	// replenished before the test can run with a third IP, so we cannot test this behaviour
	// exactly. Instead, we just test that rate limiting is effective generally, and defer
	// more thorough testing to unit tests.
	for i := 0; i < sendEventLimit*2; i++ {
		g.Go(func() error { return sendEvent(ctx, "10.11.12.13") })
		g.Go(func() error { return sendEvent(ctx, "10.11.12.14") })
		g.Go(func() error { return sendEvent(ctx, "11.11.12.15") })
	}
	err = g.Wait()
	require.Error(t, err)

	errStatus, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, errStatus.Code())
	assert.Equal(t, "rate limit exceeded", errStatus.Message())
}

func newOTLPTraceExporter(t testing.TB, srv *apmservertest.Server, options ...otlptracegrpc.Option) *otlptrace.Exporter {
	options = append(options, otlptracegrpc.WithEndpoint(serverAddr(srv)), otlptracegrpc.WithInsecure())
	exporter, err := otlptracegrpc.New(context.Background(), options...)
	require.NoError(t, err)
	t.Cleanup(func() {
		exporter.Shutdown(context.Background())
	})
	return exporter
}

func newOTLPMetricExporter(t testing.TB, srv *apmservertest.Server, options ...otlpmetricgrpc.Option) *otlpmetric.Exporter {
	options = append(options, otlpmetricgrpc.WithEndpoint(serverAddr(srv)), otlpmetricgrpc.WithInsecure())
	exporter, err := otlpmetricgrpc.New(context.Background(), options...)
	require.NoError(t, err)
	t.Cleanup(func() {
		exporter.Shutdown(context.Background())
	})
	return exporter
}

func newOTLPTracerProvider(exporter *otlptrace.Exporter, options ...sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(append([]sdktrace.TracerProviderOption{
		sdktrace.WithSyncer(exporter),
		sdktrace.WithIDGenerator(&idGeneratorFuncs{
			newIDs: func(context.Context) (trace.TraceID, trace.SpanID) {
				traceID, err := trace.TraceIDFromHex("d2acbef8b37655e48548fd9d61ad6114")
				if err != nil {
					panic(err)
				}
				spanID, err := trace.SpanIDFromHex("b3ee9be3b687a611")
				if err != nil {
					panic(err)
				}
				return traceID, spanID
			},
		}),
	}, options...)...)
}

func sendOTLPTrace(ctx context.Context, tracerProvider *sdktrace.TracerProvider) error {
	return withOTLPTracer(tracerProvider, func(tracer trace.Tracer) {
		startTime := time.Unix(123, 456)
		endTime := startTime.Add(time.Second)
		_, span := tracer.Start(ctx, "operation_name", trace.WithTimestamp(startTime), trace.WithAttributes(
			attribute.StringSlice("span_attribute_array", []string{"a", "b", "c"}),
		))
		span.End(trace.WithTimestamp(endTime))
	})
}

func withOTLPTracer(tracerProvider *sdktrace.TracerProvider, f func(trace.Tracer)) error {
	tracer := tracerProvider.Tracer("systemtest")
	f(tracer)
	return flushTracerProvider(context.Background(), tracerProvider)
}

func flushTracerProvider(ctx context.Context, tracerProvider *sdktrace.TracerProvider) error {
	if err := tracerProvider.ForceFlush(ctx); err != nil {
		return err
	}
	select {
	case err := <-otelErrors:
		return err
	default:
		return nil
	}
}

func sendOTLPMetrics(
	t testing.TB,
	ctx context.Context,
	srv *apmservertest.Server,
	aggregator export.AggregatorSelector,
	recordMetrics func(metric.MeterMust),
) error {
	exporter := newOTLPMetricExporter(t, srv)
	controller := controller.New(
		processor.New(aggregator, exporter),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(time.Minute),
	)
	if err := controller.Start(context.Background()); err != nil {
		return err
	}
	meterProvider := controller.MeterProvider()
	meter := metric.Must(meterProvider.Meter("test-meter"))
	recordMetrics(meter)

	// Stopping the controller will collect and export metrics.
	if err := controller.Stop(context.Background()); err != nil {
		return err
	}
	select {
	case err := <-otelErrors:
		return err
	default:
		return nil
	}
}

type idGeneratorFuncs struct {
	newIDs    func(context context.Context) (trace.TraceID, trace.SpanID)
	newSpanID func(ctx context.Context, traceID trace.TraceID) trace.SpanID
}

func (m *idGeneratorFuncs) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	return m.newIDs(ctx)
}

func (m *idGeneratorFuncs) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	return m.newSpanID(ctx, traceID)
}

func newLogs(body interface{}) pdata.Logs {
	logs := pdata.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	logs.ResourceLogs().At(0).Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		semconv.AttributeTelemetrySDKLanguage: pdata.NewAttributeValueString("go"),
	})
	instrumentationLogs := resourceLogs.InstrumentationLibraryLogs().AppendEmpty()
	otelLog := instrumentationLogs.Logs().AppendEmpty()
	otelLog.SetTraceID(pdata.NewTraceID([16]byte{1}))
	otelLog.SetSpanID(pdata.NewSpanID([8]byte{2}))
	otelLog.SetName("doOperation()")
	otelLog.SetSeverityNumber(pdata.SeverityNumberINFO)
	otelLog.SetSeverityText("Info")
	otelLog.SetTimestamp(pdata.NewTimestampFromTime(time.Unix(1, 0)))
	otelLog.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"key":         pdata.NewAttributeValueString("value"),
		"numeric_key": pdata.NewAttributeValueDouble(1234),
	})

	switch b := body.(type) {
	case string:
		otelLog.Body().SetStringVal(b)
	case int:
		otelLog.Body().SetIntVal(int64(b))
	case float64:
		otelLog.Body().SetDoubleVal(float64(b))
	case bool:
		otelLog.Body().SetBoolVal(b)
	}
	return logs
}
