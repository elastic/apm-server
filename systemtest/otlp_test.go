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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

var otelErrors = make(chan error, 1)

func init() {
	// otel.SetErrorHandler can only be called once per process.
	otel.SetErrorHandler(otelErrorHandlerFunc(func(err error) {
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
	err := sendOTLPTrace(ctx, srv, sdktrace.Config{})
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	}})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestOTLPGRPCMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Monitoring = &apmservertest.MonitoringConfig{
		Enabled:       true,
		MetricsPeriod: time.Duration(time.Second),
		StatePeriod:   time.Duration(time.Second),
	}
	err := srv.Start()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	aggregator := simple.NewWithHistogramDistribution([]float64{1, 100, 1000, 10000})
	err = sendOTLPMetrics(ctx, srv, aggregator, func(meter metric.MeterMust) {
		float64Counter := meter.NewFloat64Counter("float64_counter")
		float64Counter.Add(context.Background(), 1)

		// This will be dropped, as we do not support consuming histograms yet.
		int64Recorder := meter.NewInt64ValueRecorder("int64_recorder")
		int64Recorder.Record(context.Background(), 123)
	})
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "metric"},
	}})
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits, "@timestamp")

	// Make sure we report monitoring for the metrics consumer. Metric values are unit tested.
	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.otlp.grpc.metrics.consumer").Exists())
}

func TestOTLPGRPCAuth(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.SecretToken = "abc123"
	err := srv.Start()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = sendOTLPTrace(ctx, srv, sdktrace.Config{})
	assert.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	err = sendOTLPTrace(ctx, srv, sdktrace.Config{}, otlpgrpc.WithHeaders(map[string]string{"Authorization": "Bearer abc123"}))
	require.NoError(t, err)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	}})
}

func TestOTLPClientIP(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := sendOTLPTrace(ctx, srv, sdktrace.Config{})
	assert.NoError(t, err)

	err = sendOTLPTrace(ctx, srv, sdktrace.Config{
		Resource: sdkresource.NewWithAttributes(label.String("service.name", "service1")),
	})
	require.NoError(t, err)

	err = sendOTLPTrace(ctx, srv, sdktrace.Config{
		Resource: sdkresource.NewWithAttributes(
			label.String("service.name", "service2"),
			label.String("telemetry.sdk.name", "iOS"),
			label.String("telemetry.sdk.language", "swift"),
		),
	})
	require.NoError(t, err)

	// Non-iOS agent documents should have no client.ip field set.
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.TermQuery{
		Field: "service.name", Value: "service1",
	})
	assert.False(t, gjson.GetBytes(result.Hits.Hits[0].RawSource, "client.ip").Exists())

	// iOS agent documents should have a client.ip field set.
	result = systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.TermQuery{
		Field: "service.name", Value: "service2",
	})
	assert.True(t, gjson.GetBytes(result.Hits.Hits[0].RawSource, "client.ip").Exists())
}

func TestOpenTelemetryJavaMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	err := srv.Start()
	require.NoError(t, err)

	aggregator := simple.NewWithExactDistribution()
	err = sendOTLPMetrics(context.Background(), srv, aggregator, func(meter metric.MeterMust) {
		// Record well-known JVM runtime metrics, to test that they are
		// copied to their Elastic APM equivalents during ingest.
		jvmGCTime := meter.NewInt64Counter("runtime.jvm.gc.time")
		jvmGCCount := meter.NewInt64Counter("runtime.jvm.gc.count")
		jvmGCTime.Bind(label.String("gc", "G1 Young Generation")).Add(context.Background(), 123)
		jvmGCCount.Bind(label.String("gc", "G1 Young Generation")).Add(context.Background(), 1)
		jvmMemoryArea := meter.NewInt64UpDownCounter("runtime.jvm.memory.area")
		jvmMemoryArea.Bind(
			label.String("area", "heap"),
			label.String("type", "used"),
		).Add(context.Background(), 42)
	})
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm-*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "metric"},
	}})
	require.Len(t, result.Hits.Hits, 2) // one for each set of labels

	var gcHit, memoryAreaHit estest.SearchHit
	for _, hit := range result.Hits.Hits {
		require.Contains(t, hit.Source, "jvm")
		switch {
		case gjson.GetBytes(hit.RawSource, "labels.gc").Exists():
			gcHit = hit
		case gjson.GetBytes(hit.RawSource, "labels.area").Exists():
			memoryAreaHit = hit
		}
	}

	assert.Equal(t, 123.0, gjson.GetBytes(gcHit.RawSource, "runtime.jvm.gc.time").Value())
	assert.Equal(t, 1.0, gjson.GetBytes(gcHit.RawSource, "runtime.jvm.gc.count").Value())
	assert.Equal(t, map[string]interface{}{
		"gc":   "G1 Young Generation",
		"name": "G1 Young Generation",
	}, gcHit.Source["labels"])
	assert.Equal(t, 123.0, gjson.GetBytes(gcHit.RawSource, "jvm.gc.time").Value())
	assert.Equal(t, 1.0, gjson.GetBytes(gcHit.RawSource, "jvm.gc.count").Value())

	assert.Equal(t, 42.0, gjson.GetBytes(memoryAreaHit.RawSource, "runtime.jvm.memory.area").Value())
	assert.Equal(t, map[string]interface{}{
		"area": "heap",
		"type": "used",
	}, memoryAreaHit.Source["labels"])
	assert.Equal(t, 42.0, gjson.GetBytes(memoryAreaHit.RawSource, "jvm.memory.heap.used").Value())
}

func sendOTLPTrace(ctx context.Context, srv *apmservertest.Server, config sdktrace.Config, options ...otlpgrpc.Option) error {
	options = append(options, otlpgrpc.WithEndpoint(serverAddr(srv)), otlpgrpc.WithInsecure())
	driver := otlpgrpc.NewDriver(options...)
	exporter, err := otlp.NewExporter(context.Background(), driver)
	if err != nil {
		panic(err)
	}

	if config.DefaultSampler == nil {
		config.DefaultSampler = sdktrace.AlwaysSample()
	}
	if config.IDGenerator == nil {
		config.IDGenerator = &idGeneratorFuncs{
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
		}
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(config),
		sdktrace.WithBatcher(exporter),
	)

	tracer := tracerProvider.Tracer("systemtest")
	startTime := time.Unix(123, 456)
	endTime := startTime.Add(time.Second)
	_, span := tracer.Start(ctx, "operation_name", trace.WithTimestamp(startTime))
	span.End(trace.WithTimestamp(endTime))
	if err := tracerProvider.Shutdown(ctx); err != nil {
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
	ctx context.Context,
	srv *apmservertest.Server,
	aggregator export.AggregatorSelector,
	recordMetrics func(metric.MeterMust),
) error {
	driver := otlpgrpc.NewDriver(otlpgrpc.WithEndpoint(serverAddr(srv)), otlpgrpc.WithInsecure())
	exporter, err := otlp.NewExporter(context.Background(), driver)
	if err != nil {
		panic(err)
	}

	controller := controller.New(
		processor.New(aggregator, exporter),
		controller.WithPusher(exporter),
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

type otelErrorHandlerFunc func(error)

func (f otelErrorHandlerFunc) Handle(err error) {
	f(err)
}
