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

package otel

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	jaegermodel "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func TestConsumer_ConsumeTraces_Empty(t *testing.T) {
	reporter := func(ctx context.Context, p publish.PendingReq) error {
		assert.Empty(t, p.Transformables)
		return nil
	}

	consumer := Consumer{Reporter: reporter}
	traces := pdata.NewTraces()
	assert.NoError(t, consumer.ConsumeTraces(context.Background(), traces))
}

func TestConsumer_JaegerMetadata(t *testing.T) {
	jaegerBatch := jaegermodel.Batch{
		Spans: []*jaegermodel.Span{{
			StartTime: testStartTime(),
			Tags:      []jaegermodel.KeyValue{jaegerKeyValue("span.kind", "client")},
			TraceID:   jaegermodel.NewTraceID(0, 0x46467830),
			SpanID:    jaegermodel.NewSpanID(0x41414646),
		}},
	}

	for _, tc := range []struct {
		name    string
		process *jaegermodel.Process
	}{{
		name: "jaeger-version",
		process: jaegermodel.NewProcess("", []jaegermodel.KeyValue{
			jaegerKeyValue("jaeger.version", "PHP-3.4.12"),
		}),
	}, {
		name: "jaeger-no-language",
		process: jaegermodel.NewProcess("", []jaegermodel.KeyValue{
			jaegerKeyValue("jaeger.version", "3.4.12"),
		}),
	}, {
		// TODO(axw) break this down into more specific test cases.
		name: "jaeger",
		process: jaegermodel.NewProcess("foo", []jaegermodel.KeyValue{
			jaegerKeyValue("jaeger.version", "C++-3.2.1"),
			jaegerKeyValue("hostname", "host-foo"),
			jaegerKeyValue("client-uuid", "xxf0"),
			jaegerKeyValue("ip", "17.0.10.123"),
			jaegerKeyValue("foo", "bar"),
			jaegerKeyValue("peer.port", "80"),
		}),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.Len(t, req.Transformables, 1)
				events := transformAll(ctx, req)
				approveEvents(t, "metadata_"+tc.name, events)
				return nil
			}
			jaegerBatch.Process = tc.process
			traces := jaegertranslator.ProtoBatchToInternalTraces(jaegerBatch)
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))
		})
	}
}

func TestConsumer_JaegerSampleRate(t *testing.T) {
	var transformables []transform.Transformable
	reporter := func(ctx context.Context, req publish.PendingReq) error {
		transformables = append(transformables, req.Transformables...)
		events := transformAll(ctx, req)
		approveEvents(t, "jaeger_sampling_rate", events)
		return nil
	}

	jaegerBatch := jaegermodel.Batch{
		Process: jaegermodel.NewProcess("", jaegerKeyValues(
			"jaeger.version", "unknown",
			"hostname", "host-abc",
		)),
		Spans: []*jaegermodel.Span{{
			StartTime: testStartTime(),
			Duration:  testDuration(),
			Tags: []jaegermodel.KeyValue{
				jaegerKeyValue("span.kind", "server"),
				jaegerKeyValue("sampler.type", "probabilistic"),
				jaegerKeyValue("sampler.param", 0.8),
			},
		}, {
			StartTime: testStartTime(),
			Duration:  testDuration(),
			TraceID:   jaegermodel.NewTraceID(1, 1),
			References: []jaegermodel.SpanRef{{
				RefType: jaegermodel.SpanRefType_CHILD_OF,
				TraceID: jaegermodel.NewTraceID(1, 1),
				SpanID:  1,
			}},
			Tags: []jaegermodel.KeyValue{
				jaegerKeyValue("span.kind", "client"),
				jaegerKeyValue("sampler.type", "probabilistic"),
				jaegerKeyValue("sampler.param", 0.4),
			},
		}, {
			StartTime: testStartTime(),
			Duration:  testDuration(),
			Tags: []jaegermodel.KeyValue{
				jaegerKeyValue("span.kind", "server"),
				jaegerKeyValue("sampler.type", "ratelimiting"),
				jaegerKeyValue("sampler.param", 2.0), // 2 traces per second
			},
		}},
	}
	traces := jaegertranslator.ProtoBatchToInternalTraces(jaegerBatch)
	require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))

	require.Len(t, transformables, 3)
	tx1 := transformables[0].(*model.Transaction)
	tx2 := transformables[1].(*model.Transaction)
	span := transformables[2].(*model.Span)
	assert.Equal(t, 1.25 /* 1/0.8 */, tx1.RepresentativeCount)
	assert.Equal(t, 2.5 /* 1/0.4 */, span.RepresentativeCount)
	assert.Zero(t, tx2.RepresentativeCount) // not set for non-probabilistic
}

func TestConsumer_JaegerTraceID(t *testing.T) {
	var transformables []transform.Transformable
	reporter := func(ctx context.Context, req publish.PendingReq) error {
		transformables = append(transformables, req.Transformables...)
		return nil
	}

	jaegerBatch := jaegermodel.Batch{
		Process: jaegermodel.NewProcess("", jaegerKeyValues("jaeger.version", "unknown")),
		Spans: []*jaegermodel.Span{{
			TraceID: jaegermodel.NewTraceID(0, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(456),
		}, {
			TraceID: jaegermodel.NewTraceID(0x000046467830, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(789),
		}},
	}
	traces := jaegertranslator.ProtoBatchToInternalTraces(jaegerBatch)
	require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))

	require.Len(t, transformables, 2)
	assert.Equal(t, "46467830", transformables[0].(*model.Transaction).TraceID)
	assert.Equal(t, "464678300000000046467830", transformables[1].(*model.Transaction).TraceID)
}

func TestConsumer_JaegerTransaction(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spans []*jaegermodel.Span
	}{
		{
			name: "jaeger_full",
			spans: []*jaegermodel.Span{{
				StartTime:     testStartTime(),
				Duration:      testDuration(),
				TraceID:       jaegermodel.NewTraceID(0, 0x46467830),
				SpanID:        0x41414646,
				OperationName: "HTTP GET",
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("error", true),
					jaegerKeyValue("bool.a", true),
					jaegerKeyValue("double.a", 14.65),
					jaegerKeyValue("int.a", int64(148)),
					jaegerKeyValue("http.method", "get"),
					jaegerKeyValue("http.url", "http://foo.bar.com?a=12"),
					jaegerKeyValue("http.status_code", "400"),
					jaegerKeyValue("http.protocol", "HTTP/1.1"),
					jaegerKeyValue("type", "http_request"),
					jaegerKeyValue("component", "foo"),
					jaegerKeyValue("string.a.b", "some note"),
					jaegerKeyValue("service.version", "1.0"),
				},
				Logs: testJaegerLogs(),
			}},
		},
		{
			name: "jaeger_type_request",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
				References: []jaegermodel.SpanRef{{
					RefType: jaegermodel.SpanRefType_CHILD_OF,
					SpanID:  0x61626364,
				}},
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("span.kind", "server"),
					jaegerKeyValue("http.status_code", int64(500)),
					jaegerKeyValue("http.protocol", "HTTP"),
					jaegerKeyValue("http.path", "http://foo.bar.com?a=12"),
				},
			}},
		},
		{
			name: "jaeger_type_request_result",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
				References: []jaegermodel.SpanRef{{
					RefType: jaegermodel.SpanRefType_CHILD_OF,
					SpanID:  0x61626364,
				}},
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("span.kind", "server"),
					jaegerKeyValue("http.status_code", int64(200)),
					jaegerKeyValue("http.url", "localhost:8080"),
				},
			}},
		},
		{
			name: "jaeger_type_messaging",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
				References: []jaegermodel.SpanRef{{
					RefType: jaegermodel.SpanRefType_CHILD_OF,
					SpanID:  0x61626364,
				}},
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("span.kind", "server"),
					jaegerKeyValue("message_bus.destination", "queue-abc"),
				},
			}},
		},
		{
			name: "jaeger_type_component",
			spans: []*jaegermodel.Span{{
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("component", "amqp"),
				},
			}},
		},
		{
			name: "jaeger_custom",
			spans: []*jaegermodel.Span{{
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("a.b", "foo"),
				},
			}},
		},
		{
			name: "jaeger_no_attrs",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
				Duration:  testDuration(),
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("span.kind", "server"),
					jaegerKeyValue("error", true),
					jaegerKeyValue("status.code", int64(2)),
				},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := jaegermodel.Batch{
				Process: jaegermodel.NewProcess("", []jaegermodel.KeyValue{
					jaegerKeyValue("hostname", "host-abc"),
					jaegerKeyValue("jaeger.version", "unknown"),
				}),
				Spans: tc.spans,
			}
			traces := jaegertranslator.ProtoBatchToInternalTraces(batch)

			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.True(t, len(req.Transformables) >= 1)
				events := transformAll(ctx, req)
				approveEvents(t, "transaction_"+tc.name, events)
				return nil
			}
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))
		})
	}
}

func TestConsumer_JaegerSpan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spans []*jaegermodel.Span
	}{
		{
			name: "jaeger_http",
			spans: []*jaegermodel.Span{{
				OperationName: "HTTP GET",
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("error", true),
					jaegerKeyValue("hasErrors", true),
					jaegerKeyValue("double.a", 14.65),
					jaegerKeyValue("http.status_code", int64(400)),
					jaegerKeyValue("int.a", int64(148)),
					jaegerKeyValue("span.kind", "filtered"),
					jaegerKeyValue("http.url", "http://foo.bar.com?a=12"),
					jaegerKeyValue("http.method", "get"),
					jaegerKeyValue("component", "foo"),
					jaegerKeyValue("string.a.b", "some note"),
				},
				Logs: testJaegerLogs(),
			}},
		},
		{
			name: "jaeger_https_default_port",
			spans: []*jaegermodel.Span{{
				OperationName: "HTTPS GET",
				Tags: jaegerKeyValues(
					"http.url", "https://foo.bar.com:443?a=12",
				),
			}},
		},
		{
			name: "jaeger_http_status_code",
			spans: []*jaegermodel.Span{{
				OperationName: "HTTP GET",
				Tags: jaegerKeyValues(
					"http.url", "http://foo.bar.com?a=12",
					"http.method", "get",
					"http.status_code", int64(202),
				),
			}},
		},
		{
			name: "jaeger_db",
			spans: []*jaegermodel.Span{{
				Tags: jaegerKeyValues(
					"db.statement", "GET * from users",
					"db.instance", "db01",
					"db.type", "mysql",
					"db.user", "admin",
					"component", "foo",
					"peer.address", "mysql://db:3306",
					"peer.hostname", "db",
					"peer.port", int64(3306),
					"peer.service", "sql",
				),
			}},
		},
		{
			name: "jaeger_messaging",
			spans: []*jaegermodel.Span{{
				OperationName: "Message receive",
				Tags: jaegerKeyValues(
					"peer.hostname", "mq",
					"peer.port", int64(1234),
					"message_bus.destination", "queue-abc",
				),
			}},
		},
		{
			name: "jaeger_subtype_component",
			spans: []*jaegermodel.Span{{
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("component", "whatever"),
				},
			}},
		},
		{
			name:  "jaeger_custom",
			spans: []*jaegermodel.Span{{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := jaegermodel.Batch{
				Process: jaegermodel.NewProcess("", []jaegermodel.KeyValue{
					jaegerKeyValue("hostname", "host-abc"),
					jaegerKeyValue("jaeger.version", "unknown"),
				}),
				Spans: tc.spans,
			}
			for _, span := range batch.Spans {
				span.StartTime = testStartTime()
				span.Duration = testDuration()
				span.TraceID = jaegermodel.NewTraceID(0, 0x46467830)
				span.SpanID = 0x41414646
				span.References = []jaegermodel.SpanRef{{
					RefType: jaegermodel.SpanRefType_CHILD_OF,
					TraceID: jaegermodel.NewTraceID(0, 0x46467830),
					SpanID:  0x58585858,
				}}
			}
			traces := jaegertranslator.ProtoBatchToInternalTraces(batch)

			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.True(t, len(req.Transformables) >= 1)
				events := transformAll(ctx, req)
				approveEvents(t, "span_"+tc.name, events)
				return nil
			}
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))
		})
	}
}

func testJaegerLogs() []jaegermodel.Log {
	return []jaegermodel.Log{{
		// errors that can be converted to elastic errors
		Timestamp: testStartTime().Add(23 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"event", "retrying connection",
			"level", "error",
			"error", "no connection established",
		),
	}, {
		Timestamp: testStartTime().Add(43 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"event", "no user.ID given",
			"level", "error",
			"message", "nullPointer exception",
			"isbool", true,
		),
	}, {
		Timestamp: testStartTime().Add(66 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"error", "no connection established",
		),
	}, {
		Timestamp: testStartTime().Add(66 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"error.object", "no connection established",
		),
	}, {
		Timestamp: testStartTime().Add(66 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"error.kind", "DBClosedException",
		),
	}, {
		Timestamp: testStartTime().Add(66 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"event", "error",
			"message", "no connection established",
		),
	}, {
		// non errors
		Timestamp: testStartTime().Add(15 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"event", "baggage",
			"isValid", false,
		),
	}, {
		Timestamp: testStartTime().Add(65 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"event", "retrying connection",
			"level", "info",
		),
	}, {
		// errors not convertible to elastic errors
		Timestamp: testStartTime().Add(67 * time.Nanosecond),
		Fields: jaegerKeyValues(
			"level", "error",
		),
	}}
}

func testStartTime() time.Time {
	return time.Unix(1576500418, 768068)
}

func testDuration() time.Duration {
	return 79 * time.Second
}

func transformAll(ctx context.Context, p publish.PendingReq) []beat.Event {
	var events []beat.Event
	for _, transformable := range p.Transformables {
		events = append(events, transformable.Transform(ctx, &transform.Config{DataStreams: true})...)
	}
	return events
}

func approveEvents(t testing.TB, name string, events []beat.Event) {
	t.Helper()
	docs := beatertest.EncodeEventDocs(events...)
	approvaltest.ApproveEventDocs(t, filepath.Join("test_approved", name), docs)
}

func jaegerKeyValues(kv ...interface{}) []jaegermodel.KeyValue {
	if len(kv)%2 != 0 {
		panic("even number of args expected")
	}
	out := make([]jaegermodel.KeyValue, len(kv)/2)
	for i := range out {
		k := kv[2*i].(string)
		v := kv[2*i+1]
		out[i] = jaegerKeyValue(k, v)
	}
	return out
}

func jaegerKeyValue(k string, v interface{}) jaegermodel.KeyValue {
	kv := jaegermodel.KeyValue{Key: k}
	switch v := v.(type) {
	case string:
		kv.VType = jaegermodel.ValueType_STRING
		kv.VStr = v
	case float64:
		kv.VType = jaegermodel.ValueType_FLOAT64
		kv.VFloat64 = v
	case int64:
		kv.VType = jaegermodel.ValueType_INT64
		kv.VInt64 = v
	case bool:
		kv.VType = jaegermodel.ValueType_BOOL
		kv.VBool = v
	default:
		panic(fmt.Errorf("unhandled %q value type %#v", k, v))
	}
	return kv
}
