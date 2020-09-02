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
	"path/filepath"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func TestConsumer_ConsumeTraceData(t *testing.T) {
	for _, tc := range []struct {
		name string
		td   consumerdata.TraceData
	}{
		{name: "empty", td: consumerdata.TraceData{}},
		{name: "emptytrace", td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Node:         &commonpb.Node{},
			Resource:     &resourcepb.Resource{},
			Spans:        []*tracepb.Span{}}},
		{name: "span", td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans: []*tracepb.Span{
				{Kind: tracepb.Span_SERVER, StartTime: &timestamp.Timestamp{Seconds: 1576500418, Nanos: 768068}},
				{ParentSpanId: []byte{0, 0, 0, 0, 70, 70, 48, 88}, StartTime: &timestamp.Timestamp{Seconds: 1576500418, Nanos: 768068}},
			}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := func(ctx context.Context, p publish.PendingReq) error {
				events := transformAll(ctx, p)
				approveEvents(t, "consume_"+tc.name, events)
				return nil
			}
			consumer := Consumer{Reporter: reporter}
			assert.NoError(t, consumer.ConsumeTraceData(context.Background(), tc.td))
		})
	}
}

func TestConsumer_Metadata(t *testing.T) {
	spans := []*tracepb.Span{{
		TraceId:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48},
		SpanId:    []byte{0, 0, 0, 0, 65, 65, 70, 70},
		Kind:      tracepb.Span_CLIENT,
		StartTime: testStartTime(),
	}}
	for _, tc := range []struct {
		name string
		td   consumerdata.TraceData
	}{{
		name: "jaeger",
		td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans:        spans,
			Node: &commonpb.Node{
				Identifier: &commonpb.ProcessIdentifier{
					HostName:       "host-foo",
					Pid:            107892,
					StartTimestamp: testStartTime()},
				LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "Jaeger-C++-3.2.1"},
				ServiceInfo: &commonpb.ServiceInfo{Name: "foo"},
				Attributes:  map[string]string{"client-uuid": "xxf0", "ip": "17.0.10.123", "foo": "bar"},
			},
			Resource: &resourcepb.Resource{
				Type:   "request",
				Labels: map[string]string{"a": "b", "c": "d"},
			},
		},
	}, {
		name: "jaeger-version",
		td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans:        spans,
			Node:         &commonpb.Node{LibraryInfo: &commonpb.LibraryInfo{Language: 7, ExporterVersion: "Jaeger-3.4.12"}},
		},
	}, {
		name: "jaeger-no-language",
		td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans:        spans,
			Node:         &commonpb.Node{LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "Jaeger-3.4.12"}},
		},
	}, {
		name: "jaeger_minimal",
		td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans:        spans,
			Node: &commonpb.Node{
				Identifier:  &commonpb.ProcessIdentifier{},
				LibraryInfo: &commonpb.LibraryInfo{},
				ServiceInfo: &commonpb.ServiceInfo{},
			},
		},
	}, {
		name: "jaeger_full-traceid",
		td: consumerdata.TraceData{
			SourceFormat: "jaeger",
			Spans: []*tracepb.Span{{
				TraceId:   []byte{0, 0, 0, 0, 70, 70, 120, 48, 0, 0, 0, 0, 70, 70, 120, 48},
				SpanId:    []byte{0, 0, 0, 0, 65, 65, 70, 70},
				Kind:      tracepb.Span_CLIENT,
				StartTime: testStartTime(),
			}},
			Node: &commonpb.Node{
				Identifier:  &commonpb.ProcessIdentifier{},
				LibraryInfo: &commonpb.LibraryInfo{},
				ServiceInfo: &commonpb.ServiceInfo{},
			},
		},
	}, {
		name: "minimal",
		td:   consumerdata.TraceData{SourceFormat: "foo", Spans: spans},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.Len(t, req.Transformables, 1)
				events := transformAll(ctx, req)
				approveEvents(t, "metadata_"+tc.name, events)
				return nil
			}
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraceData(context.Background(), tc.td))
		})
	}
}

func TestConsumer_Transaction(t *testing.T) {
	for _, tc := range []struct {
		name string
		td   consumerdata.TraceData
	}{
		{name: "jaeger_full",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					TraceId:                 []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48},
					SpanId:                  []byte{0, 0, 0, 0, 65, 65, 70, 70},
					StartTime:               testStartTime(),
					EndTime:                 testEndTime(),
					Name:                    testTruncatableString("HTTP GET"),
					ChildSpanCount:          testIntToWrappersUint32(10),
					SameProcessAsParentSpan: testBoolToWrappersBool(true),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"error":            testAttributeBoolValue(true),
						"bool.a":           testAttributeBoolValue(true),
						"double.a":         testAttributeDoubleValue(14.65),
						"int.a":            testAttributeIntValue(148),
						"span.kind":        testAttributeStringValue("http request"),
						"http.method":      testAttributeStringValue("get"),
						"http.url":         testAttributeStringValue("http://foo.bar.com?a=12"),
						"http.status_code": testAttributeStringValue("400"),
						"http.protocol":    testAttributeStringValue("HTTP/1.1"),
						"type":             testAttributeStringValue("http_request"),
						"component":        testAttributeStringValue("foo"),
						"string.a.b":       testAttributeStringValue("some note"),
						"service.version":  testAttributeStringValue("1.0"),
					}},
					TimeEvents: testTimeEvents(),
				}}}},
		{name: "jaeger_type_request",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					ParentSpanId: []byte{0, 0, 0, 0, 97, 98, 99, 100}, Kind: tracepb.Span_SERVER,
					StartTime: testStartTime(),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"http.status_code": testAttributeIntValue(500),
						"http.protocol":    testAttributeStringValue("HTTP"),
						"http.path":        testAttributeStringValue("http://foo.bar.com?a=12"),
					}}}}}},
		{name: "jaeger_type_request_result",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					ParentSpanId: []byte{0, 0, 0, 0, 97, 98, 99, 100}, Kind: tracepb.Span_SERVER,
					StartTime: testStartTime(),
					Status:    &tracepb.Status{Code: 200},
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"http.url": testAttributeStringValue("localhost:8080"),
					}}}}}},
		{name: "jaeger_type_messaging",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					ParentSpanId: []byte{0, 0, 0, 0, 97, 98, 99, 100}, Kind: tracepb.Span_SERVER,
					StartTime: testStartTime(),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"message_bus.destination": testAttributeStringValue("queue-abc"),
					}}}}}},
		{name: "jaeger_type_component",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"component": testAttributeStringValue("amqp"),
					}}}}}},
		{name: "jaeger_custom",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Spans: []*tracepb.Span{{Attributes: &tracepb.Span_Attributes{
					AttributeMap: map[string]*tracepb.AttributeValue{
						"a.b": testAttributeStringValue("foo")}}}},
				Node: &commonpb.Node{
					Identifier:  &commonpb.ProcessIdentifier{},
					LibraryInfo: &commonpb.LibraryInfo{},
					ServiceInfo: &commonpb.ServiceInfo{},
				}}},
		{name: "jaeger_no_attrs",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					Kind:      tracepb.Span_SERVER,
					StartTime: testStartTime(), EndTime: testEndTime(),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"error": testAttributeBoolValue(true),
					}},
					Status: &tracepb.Status{Code: 500}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.True(t, len(req.Transformables) >= 1)
				events := transformAll(ctx, req)
				approveEvents(t, "transaction_"+tc.name, events)
				return nil
			}
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraceData(context.Background(), tc.td))
		})
	}
}

func TestConsumer_SampleRate(t *testing.T) {
	var transformables []transform.Transformable
	reporter := func(ctx context.Context, req publish.PendingReq) error {
		transformables = append(transformables, req.Transformables...)
		events := transformAll(ctx, req)
		approveEvents(t, "jaeger_sampling_rate", events)
		return nil
	}
	require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraceData(context.Background(), consumerdata.TraceData{
		SourceFormat: "jaeger",
		Node: &commonpb.Node{
			Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"},
		},
		Spans: []*tracepb.Span{{
			Kind:      tracepb.Span_SERVER,
			StartTime: testStartTime(), EndTime: testEndTime(),
			Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
				"sampler.type":  testAttributeStringValue("probabilistic"),
				"sampler.param": testAttributeDoubleValue(0.8),
			}},
		}, {
			Kind:      tracepb.Span_CLIENT,
			StartTime: testStartTime(), EndTime: testEndTime(),
			ParentSpanId: []byte{1},
			Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
				"sampler.type":  testAttributeStringValue("probabilistic"),
				"sampler.param": testAttributeDoubleValue(0.4),
			}},
		}},
	}))

	require.Len(t, transformables, 2)
	tx := transformables[0].(*model.Transaction)
	span := transformables[1].(*model.Span)
	assert.Equal(t, 1.25 /* 1/0.8 */, tx.RepresentativeCount)
	assert.Equal(t, 2.5 /* 1/0.4 */, span.RepresentativeCount)
}

func TestConsumer_Span(t *testing.T) {
	for _, tc := range []struct {
		name string
		td   consumerdata.TraceData
	}{
		{name: "jaeger_http",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					TraceId: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48}, SpanId: []byte{0, 0, 0, 0, 65, 65, 70, 70}, ParentSpanId: []byte{0, 0, 0, 0, 88, 88, 88, 88},
					StartTime: testStartTime(), EndTime: testEndTime(),
					Name: testTruncatableString("HTTP GET"),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"error":            testAttributeBoolValue(true),
						"hasErrors":        testAttributeBoolValue(true),
						"double.a":         testAttributeDoubleValue(14.65),
						"http.status_code": testAttributeIntValue(400),
						"int.a":            testAttributeIntValue(148),
						"span.kind":        testAttributeStringValue("filtered"),
						"http.url":         testAttributeStringValue("http://foo.bar.com?a=12"),
						"http.method":      testAttributeStringValue("get"),
						"component":        testAttributeStringValue("foo"),
						"string.a.b":       testAttributeStringValue("some note"),
					}},
					TimeEvents: testTimeEvents(),
				}}}},
		{name: "jaeger_https_default_port",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					TraceId: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48}, SpanId: []byte{0, 0, 0, 0, 65, 65, 70, 70}, ParentSpanId: []byte{0, 0, 0, 0, 88, 88, 88, 88},
					StartTime: testStartTime(), EndTime: testEndTime(),
					Name: testTruncatableString("HTTPS GET"),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"http.url": testAttributeStringValue("https://foo.bar.com:443?a=12"),
					}},
				}}}},
		{name: "jaeger_http_status_code",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					TraceId: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48}, SpanId: []byte{0, 0, 0, 0, 65, 65, 70, 70}, ParentSpanId: []byte{0, 0, 0, 0, 88, 88, 88, 88},
					StartTime: testStartTime(), EndTime: testEndTime(),
					Name: testTruncatableString("HTTP GET"),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"http.url":    testAttributeStringValue("http://foo.bar.com?a=12"),
						"http.method": testAttributeStringValue("get"),
					}},
					Status: &tracepb.Status{Code: 202},
				}}}},
		{name: "jaeger_db",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					ParentSpanId: []byte{0, 0, 0, 0, 97, 98, 99, 100}, Kind: tracepb.Span_CLIENT,
					StartTime: testStartTime(), Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"db.statement":  testAttributeStringValue("GET * from users"),
						"db.instance":   testAttributeStringValue("db01"),
						"db.type":       testAttributeStringValue("mysql"),
						"db.user":       testAttributeStringValue("admin"),
						"component":     testAttributeStringValue("foo"),
						"peer.address":  testAttributeStringValue("mysql://db:3306"),
						"peer.hostname": testAttributeStringValue("db"),
						"peer.port":     testAttributeIntValue(3306),
						"peer.service":  testAttributeStringValue("sql"),
					}},
				}}}},
		{name: "jaeger_messaging",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					TraceId: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 70, 70, 120, 48}, SpanId: []byte{0, 0, 0, 0, 65, 65, 70, 70}, ParentSpanId: []byte{0, 0, 0, 0, 88, 88, 88, 88},
					StartTime: testStartTime(), EndTime: testEndTime(),
					Name: testTruncatableString("Message receive"),
					Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
						"peer.hostname":           testAttributeStringValue("mq"),
						"peer.port":               testAttributeIntValue(1234),
						"message_bus.destination": testAttributeStringValue("queue-abc"),
					}},
					Status: &tracepb.Status{Code: 202},
				}}}},
		{name: "jaeger_custom",
			td: consumerdata.TraceData{SourceFormat: "jaeger",
				Node: &commonpb.Node{Identifier: &commonpb.ProcessIdentifier{HostName: "host-abc"}},
				Spans: []*tracepb.Span{{
					ParentSpanId: []byte{0, 0, 0, 0, 97, 98, 99, 100}, Kind: tracepb.Span_CLIENT,
					StartTime: testStartTime()}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := func(ctx context.Context, req publish.PendingReq) error {
				require.True(t, len(req.Transformables) >= 1)
				events := transformAll(ctx, req)
				approveEvents(t, "span_"+tc.name, events)
				return nil
			}
			require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraceData(context.Background(), tc.td))
		})
	}
}

func testTimeEvents() *tracepb.Span_TimeEvents {
	return &tracepb.Span_TimeEvents{TimeEvent: []*tracepb.Span_TimeEvent{
		// errors that can be converted to elastic errors
		{Time: testTimeStamp(testStartTime(), 23),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"event": testAttributeStringValue("retrying connection"),
					"level": testAttributeStringValue("error"),
					"error": testAttributeStringValue("no connection established"),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 43),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"event":   testAttributeStringValue("no user.ID given"),
					"message": testAttributeStringValue("nullPointer exception"),
					"level":   testAttributeStringValue("error"),
					"isbool":  testAttributeBoolValue(true),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 66),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"error": testAttributeStringValue("no connection established"),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 66),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"error.object": testAttributeStringValue("no connection established"),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 66),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"error.kind": testAttributeStringValue("DBClosedException"),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 66),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"event":   testAttributeStringValue("error"),
					"message": testAttributeStringValue("no connection established"),
				}}}}},
		// no errors
		{Time: testTimeStamp(testStartTime(), 15),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"event":   testAttributeStringValue("baggage"),
					"isValid": testAttributeBoolValue(false),
				}}}}},
		{Time: testTimeStamp(testStartTime(), 65),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"event": testAttributeStringValue("retrying connection"),
					"level": testAttributeStringValue("info"),
				}}}}},
		// errors not convertable to elastic errors
		{Time: testTimeStamp(testStartTime(), 67),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: &tracepb.Span_TimeEvent_Annotation{
				Attributes: &tracepb.Span_Attributes{AttributeMap: map[string]*tracepb.AttributeValue{
					"level": testAttributeStringValue("error"),
				}}}}}}}
}

func testStartTime() *timestamp.Timestamp {
	return &timestamp.Timestamp{Seconds: 1576500418, Nanos: 768068}
}

func testEndTime() *timestamp.Timestamp {
	return &timestamp.Timestamp{Seconds: 1576500497, Nanos: 768068}
}

func testTimeStamp(t *timestamp.Timestamp, addNanos int32) *timestamp.Timestamp {
	return &timestamp.Timestamp{Seconds: t.GetSeconds(), Nanos: t.GetNanos() + addNanos}
}

func testIntToWrappersUint32(n int) *wrappers.UInt32Value {
	return &wrappers.UInt32Value{Value: uint32(n)}
}

func testBoolToWrappersBool(b bool) *wrappers.BoolValue {
	return &wrappers.BoolValue{Value: b}
}

func testTruncatableString(s string) *tracepb.TruncatableString {
	return &tracepb.TruncatableString{Value: s}
}

func testAttributeIntValue(n int) *tracepb.AttributeValue {
	return &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{IntValue: int64(n)}}
}
func testAttributeBoolValue(b bool) *tracepb.AttributeValue {
	return &tracepb.AttributeValue{Value: &tracepb.AttributeValue_BoolValue{BoolValue: b}}
}
func testAttributeDoubleValue(f float64) *tracepb.AttributeValue {
	return &tracepb.AttributeValue{Value: &tracepb.AttributeValue_DoubleValue{DoubleValue: f}}
}

func testAttributeStringValue(s string) *tracepb.AttributeValue {
	return &tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{StringValue: testTruncatableString(s)}}
}

func transformAll(ctx context.Context, p publish.PendingReq) []beat.Event {
	var events []beat.Event
	for _, transformable := range p.Transformables {
		events = append(events, transformable.Transform(ctx, &transform.Config{})...)
	}
	return events
}

func approveEvents(t testing.TB, name string, events []beat.Event) {
	docs := beatertest.EncodeEventDocs(events...)
	approvaltest.ApproveEventDocs(t, file(name), docs)
}

func file(f string) string {
	return filepath.Join("test_approved", f)
}
