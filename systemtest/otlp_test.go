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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
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

	err := sendOTLPTrace(ctx, newOTLPTracerProvider(newOTLPExporter(t, srv), sdktrace.WithResource(
		resource.Merge(resource.Default(), sdkresource.NewWithAttributes(
			attribute.Array("resource_attribute_array", []string{"a", "b"}),
		)),
	)))
	require.NoError(t, err)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.BoolQuery{Filter: []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	}})
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

		int64Recorder := meter.NewInt64ValueRecorder("int64_recorder")
		int64Recorder.Record(context.Background(), 1)
		int64Recorder.Record(context.Background(), 123)
		int64Recorder.Record(context.Background(), 1024)
		int64Recorder.Record(context.Background(), 20000)
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

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(newOTLPExporter(t, srv)))
	assert.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(newOTLPExporter(t, srv, otlpgrpc.WithHeaders(map[string]string{
		"Authorization": "Bearer abc123",
	}))))
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

	exporter := newOTLPExporter(t, srv)
	err := sendOTLPTrace(ctx, newOTLPTracerProvider(exporter))
	assert.NoError(t, err)

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(
		sdkresource.NewWithAttributes(attribute.String("service.name", "service1")),
	)))
	require.NoError(t, err)

	err = sendOTLPTrace(ctx, newOTLPTracerProvider(exporter, sdktrace.WithResource(
		sdkresource.NewWithAttributes(
			attribute.String("service.name", "service2"),
			attribute.String("telemetry.sdk.name", "iOS"),
			attribute.String("telemetry.sdk.language", "swift"),
		),
	)))
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

func newOTLPExporter(t testing.TB, srv *apmservertest.Server, options ...otlpgrpc.Option) *otlp.Exporter {
	options = append(options, otlpgrpc.WithEndpoint(serverAddr(srv)), otlpgrpc.WithInsecure())
	driver := otlpgrpc.NewDriver(options...)
	exporter, err := otlp.NewExporter(context.Background(), driver)
	require.NoError(t, err)
	t.Cleanup(func() {
		exporter.Shutdown(context.Background())
	})
	return exporter
}

func newOTLPTracerProvider(exporter *otlp.Exporter, options ...sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
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
	tracer := tracerProvider.Tracer("systemtest")
	startTime := time.Unix(123, 456)
	endTime := startTime.Add(time.Second)
	_, span := tracer.Start(ctx, "operation_name", trace.WithTimestamp(startTime), trace.WithAttributes(
		attribute.Array("span_attribute_array", []string{"a", "b", "c"}),
	))
	span.End(trace.WithTimestamp(endTime))
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
	exporter := newOTLPExporter(t, srv)
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

type otelErrorHandlerFunc func(error)

func (f otelErrorHandlerFunc) Handle(err error) {
	f(err)
}
