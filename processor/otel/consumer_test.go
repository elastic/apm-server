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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	jaegermodel "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"

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

func TestOutcome(t *testing.T) {
	test := func(t *testing.T, expectedOutcome, expectedResult string, statusCode pdata.StatusCode) {
		t.Helper()

		traces, spans := newTracesSpans()
		otelSpan1 := pdata.NewSpan()
		otelSpan1.SetTraceID(pdata.NewTraceID([16]byte{1}))
		otelSpan1.SetSpanID(pdata.NewSpanID([8]byte{2}))
		otelSpan1.Status().SetCode(statusCode)
		otelSpan2 := pdata.NewSpan()
		otelSpan2.SetTraceID(pdata.NewTraceID([16]byte{1}))
		otelSpan2.SetSpanID(pdata.NewSpanID([8]byte{2}))
		otelSpan2.SetParentSpanID(pdata.NewSpanID([8]byte{3}))
		otelSpan2.Status().SetCode(statusCode)

		spans.Spans().Append(otelSpan1)
		spans.Spans().Append(otelSpan2)
		events := transformTraces(t, traces)
		require.Len(t, events, 2)

		tx := events[0].(*model.Transaction)
		span := events[1].(*model.Span)
		assert.Equal(t, expectedOutcome, tx.Outcome)
		assert.Equal(t, expectedResult, tx.Result)
		assert.Equal(t, expectedOutcome, span.Outcome)
	}

	test(t, "unknown", "", pdata.StatusCodeUnset)
	test(t, "success", "Success", pdata.StatusCodeOk)
	test(t, "failure", "Error", pdata.StatusCodeError)
}

func TestHTTPTransactionURL(t *testing.T) {
	test := func(t *testing.T, expected *model.URL, attrs map[string]pdata.AttributeValue) {
		t.Helper()
		tx := transformTransactionWithAttributes(t, attrs)
		assert.Equal(t, expected, tx.URL)
	}

	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("/foo?bar"),
			Full:     newString("https://testing.invalid:80/foo?bar"),
			Path:     newString("/foo"),
			Query:    newString("bar"),
			Domain:   newString("testing.invalid"),
			Port:     newInt(80),
		}, map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_servername_nethostport_target", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("/foo?bar"),
			Full:     newString("https://testing.invalid:80/foo?bar"),
			Path:     newString("/foo"),
			Query:    newString("bar"),
			Domain:   newString("testing.invalid"),
			Port:     newInt(80),
		}, map[string]pdata.AttributeValue{
			"http.scheme":      pdata.NewAttributeValueString("https"),
			"http.server_name": pdata.NewAttributeValueString("testing.invalid"),
			"net.host.port":    pdata.NewAttributeValueInt(80),
			"http.target":      pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_nethostname_nethostport_target", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("/foo?bar"),
			Full:     newString("https://testing.invalid:80/foo?bar"),
			Path:     newString("/foo"),
			Query:    newString("bar"),
			Domain:   newString("testing.invalid"),
			Port:     newInt(80),
		}, map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.host.name": pdata.NewAttributeValueString("testing.invalid"),
			"net.host.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("http.url", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("https://testing.invalid:80/foo?bar"),
			Full:     newString("https://testing.invalid:80/foo?bar"),
			Path:     newString("/foo"),
			Query:    newString("bar"),
			Domain:   newString("testing.invalid"),
			Port:     newInt(80),
		}, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:80/foo?bar"),
		})
	})
	t.Run("host_no_port", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("/foo"),
			Full:     newString("https://testing.invalid/foo"),
			Path:     newString("/foo"),
			Domain:   newString("testing.invalid"),
		}, map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid"),
			"http.target": pdata.NewAttributeValueString("/foo"),
		})
	})
	t.Run("ipv6_host_no_port", func(t *testing.T) {
		test(t, &model.URL{
			Scheme:   newString("https"),
			Original: newString("/foo"),
			Full:     newString("https://[::1]/foo"),
			Path:     newString("/foo"),
			Domain:   newString("::1"),
		}, map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("[::1]"),
			"http.target": pdata.NewAttributeValueString("/foo"),
		})
	})
	t.Run("default_scheme", func(t *testing.T) {
		// scheme is set to "http" if it can't be deduced from attributes.
		test(t, &model.URL{
			Scheme:   newString("http"),
			Original: newString("/foo"),
			Full:     newString("http://testing.invalid/foo"),
			Path:     newString("/foo"),
			Domain:   newString("testing.invalid"),
		}, map[string]pdata.AttributeValue{
			"http.host":   pdata.NewAttributeValueString("testing.invalid"),
			"http.target": pdata.NewAttributeValueString("/foo"),
		})
	})
}

func TestHTTPSpanURL(t *testing.T) {
	test := func(t *testing.T, expected string, attrs map[string]pdata.AttributeValue) {
		t.Helper()
		span := transformSpanWithAttributes(t, attrs)
		require.NotNil(t, span.HTTP)
		require.NotNil(t, span.HTTP.URL)
		assert.Equal(t, expected, *span.HTTP.URL)
	}

	t.Run("host.url", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:80/foo?bar"),
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeername_netpeerport_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.name": pdata.NewAttributeValueString("testing.invalid"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"), // net.peer.name preferred
			"net.peer.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeerip_netpeerport_target", func(t *testing.T) {
		test(t, "https://[::1]:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"),
			"net.peer.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("default_scheme", func(t *testing.T) {
		// scheme is set to "http" if it can't be deduced from attributes.
		test(t, "http://testing.invalid/foo", map[string]pdata.AttributeValue{
			"http.host":   pdata.NewAttributeValueString("testing.invalid"),
			"http.target": pdata.NewAttributeValueString("/foo"),
		})
	})
}

func TestHTTPSpanDestination(t *testing.T) {
	test := func(t *testing.T, expectedDestination *model.Destination, expectedDestinationService *model.DestinationService, attrs map[string]pdata.AttributeValue) {
		t.Helper()
		span := transformSpanWithAttributes(t, attrs)
		assert.Equal(t, expectedDestination, span.Destination)
		assert.Equal(t, expectedDestinationService, span.DestinationService)
	}

	t.Run("url_default_port_specified", func(t *testing.T) {
		test(t, &model.Destination{
			Address: newString("testing.invalid"),
			Port:    newInt(443),
		}, &model.DestinationService{
			Type:     newString("external"),
			Name:     newString("https://testing.invalid"),
			Resource: newString("testing.invalid:443"),
		}, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:443/foo?bar"),
		})
	})
	t.Run("url_port_scheme", func(t *testing.T) {
		test(t, &model.Destination{
			Address: newString("testing.invalid"),
			Port:    newInt(443),
		}, &model.DestinationService{
			Type:     newString("external"),
			Name:     newString("https://testing.invalid"),
			Resource: newString("testing.invalid:443"),
		}, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid/foo?bar"),
		})
	})
	t.Run("url_non_default_port", func(t *testing.T) {
		test(t, &model.Destination{
			Address: newString("testing.invalid"),
			Port:    newInt(444),
		}, &model.DestinationService{
			Type:     newString("external"),
			Name:     newString("https://testing.invalid:444"),
			Resource: newString("testing.invalid:444"),
		}, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:444/foo?bar"),
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, &model.Destination{
			Address: newString("testing.invalid"),
			Port:    newInt(444),
		}, &model.DestinationService{
			Type:     newString("external"),
			Name:     newString("https://testing.invalid:444"),
			Resource: newString("testing.invalid:444"),
		}, map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:444"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeername_nethostport_target", func(t *testing.T) {
		test(t, &model.Destination{
			Address: newString("::1"),
			Port:    newInt(444),
		}, &model.DestinationService{
			Type:     newString("external"),
			Name:     newString("https://[::1]:444"),
			Resource: newString("[::1]:444"),
		}, map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"),
			"net.peer.port": pdata.NewAttributeValueInt(444),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
}

func TestHTTPTransactionRequestSocketRemoteAddr(t *testing.T) {
	test := func(t *testing.T, expected string, attrs map[string]pdata.AttributeValue) {
		// "http.method" is a required attribute for HTTP spans,
		// and its presence causes the transaction's HTTP request
		// context to be built.
		attrs["http.method"] = pdata.NewAttributeValueString("POST")

		tx := transformTransactionWithAttributes(t, attrs)
		require.NotNil(t, tx.HTTP)
		require.NotNil(t, tx.HTTP.Request)
		require.NotNil(t, tx.HTTP.Request.Socket)
		assert.Equal(t, &expected, tx.HTTP.Request.Socket.RemoteAddress)
	}

	t.Run("net.peer.ip_port", func(t *testing.T) {
		test(t, "192.168.0.1:1234", map[string]pdata.AttributeValue{
			"net.peer.ip":   pdata.NewAttributeValueString("192.168.0.1"),
			"net.peer.port": pdata.NewAttributeValueInt(1234),
		})
	})
	t.Run("net.peer.ip", func(t *testing.T) {
		test(t, "192.168.0.1", map[string]pdata.AttributeValue{
			"net.peer.ip": pdata.NewAttributeValueString("192.168.0.1"),
		})
	})
	t.Run("http.remote_addr", func(t *testing.T) {
		test(t, "192.168.0.1:1234", map[string]pdata.AttributeValue{
			"http.remote_addr": pdata.NewAttributeValueString("192.168.0.1:1234"),
		})
	})
	t.Run("http.remote_addr_no_port", func(t *testing.T) {
		test(t, "192.168.0.1", map[string]pdata.AttributeValue{
			"http.remote_addr": pdata.NewAttributeValueString("192.168.0.1"),
		})
	})
}

func TestHTTPTransactionFlavor(t *testing.T) {
	tx := transformTransactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.flavor": pdata.NewAttributeValueString("1.1"),
	})
	assert.Equal(t, newString("1.1"), tx.HTTP.Version)
}

func TestHTTPTransactionUserAgent(t *testing.T) {
	tx := transformTransactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.user_agent": pdata.NewAttributeValueString("Foo/bar (baz)"),
	})
	assert.Equal(t, model.UserAgent{Original: "Foo/bar (baz)"}, tx.Metadata.UserAgent)
}

func TestHTTPTransactionClientIP(t *testing.T) {
	tx := transformTransactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.client_ip": pdata.NewAttributeValueString("256.257.258.259"),
	})
	assert.Equal(t, net.ParseIP("256.257.258.259"), tx.Metadata.Client.IP)
}

func TestHTTPTransactionStatusCode(t *testing.T) {
	tx := transformTransactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.status_code": pdata.NewAttributeValueInt(200),
	})
	assert.Equal(t, newInt(200), tx.HTTP.Response.StatusCode)
}

func TestDatabaseSpan(t *testing.T) {
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/database.md#mysql
	connectionString := "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;"
	span := transformSpanWithAttributes(t, map[string]pdata.AttributeValue{
		"db.system":            pdata.NewAttributeValueString("mysql"),
		"db.connection_string": pdata.NewAttributeValueString(connectionString),
		"db.user":              pdata.NewAttributeValueString("billing_user"),
		"db.name":              pdata.NewAttributeValueString("ShopDb"),
		"db.statement":         pdata.NewAttributeValueString("SELECT * FROM orders WHERE order_id = 'o4711'"),
		"net.peer.name":        pdata.NewAttributeValueString("shopdb.example.com"),
		"net.peer.ip":          pdata.NewAttributeValueString("192.0.2.12"),
		"net.peer.port":        pdata.NewAttributeValueInt(3306),
		"net.transport":        pdata.NewAttributeValueString("IP.TCP"),
	})

	assert.Equal(t, "db", span.Type)
	assert.Equal(t, newString("mysql"), span.Subtype)
	assert.Nil(t, span.Action)

	assert.Equal(t, &model.DB{
		Instance:  newString("ShopDb"),
		Statement: newString("SELECT * FROM orders WHERE order_id = 'o4711'"),
		Type:      newString("mysql"),
		UserName:  newString("billing_user"),
	}, span.DB)

	assert.Equal(t, common.MapStr{
		"db_connection_string": connectionString,
		"net_transport":        "IP.TCP",
	}, span.Labels)

	assert.Equal(t, &model.Destination{
		Address: newString("shopdb.example.com"),
		Port:    newInt(3306),
	}, span.Destination)

	assert.Equal(t, &model.DestinationService{
		Type:     newString("db"),
		Name:     newString("mysql"),
		Resource: newString("mysql"),
	}, span.DestinationService)
}

func TestInstrumentationLibrary(t *testing.T) {
	traces, spans := newTracesSpans()
	spans.InstrumentationLibrary().SetName("library-name")
	spans.InstrumentationLibrary().SetVersion("1.2.3")
	otelSpan := pdata.NewSpan()
	otelSpan.SetTraceID(pdata.NewTraceID([16]byte{1}))
	otelSpan.SetSpanID(pdata.NewSpanID([8]byte{2}))
	spans.Spans().Append(otelSpan)
	events := transformTraces(t, traces)
	require.Len(t, events, 1)
	tx := events[0].(*model.Transaction)

	assert.Equal(t, "library-name", tx.Metadata.Service.Framework.Name)
	assert.Equal(t, "1.2.3", tx.Metadata.Service.Framework.Version)
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
	assert.Equal(t, "00000000000000000000000046467830", transformables[0].(*model.Transaction).TraceID)
	assert.Equal(t, "00000000464678300000000046467830", transformables[1].(*model.Transaction).TraceID)
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

func transformTransactionWithAttributes(t *testing.T, attrs map[string]pdata.AttributeValue) *model.Transaction {
	traces, spans := newTracesSpans()
	otelSpan := pdata.NewSpan()
	otelSpan.SetTraceID(pdata.NewTraceID([16]byte{1}))
	otelSpan.SetSpanID(pdata.NewSpanID([8]byte{2}))
	otelSpan.Attributes().InitFromMap(attrs)
	spans.Spans().Append(otelSpan)
	events := transformTraces(t, traces)
	require.Len(t, events, 1)
	return events[0].(*model.Transaction)
}

func transformSpanWithAttributes(t *testing.T, attrs map[string]pdata.AttributeValue) *model.Span {
	traces, spans := newTracesSpans()
	otelSpan := pdata.NewSpan()
	otelSpan.SetTraceID(pdata.NewTraceID([16]byte{1}))
	otelSpan.SetSpanID(pdata.NewSpanID([8]byte{2}))
	otelSpan.SetParentSpanID(pdata.NewSpanID([8]byte{3}))
	otelSpan.Attributes().InitFromMap(attrs)
	spans.Spans().Append(otelSpan)
	events := transformTraces(t, traces)
	require.Len(t, events, 1)
	return events[0].(*model.Span)
}

func transformTraces(t *testing.T, traces pdata.Traces) []transform.Transformable {
	var events []transform.Transformable
	reporter := func(ctx context.Context, req publish.PendingReq) error {
		events = append(events, req.Transformables...)
		return nil
	}
	require.NoError(t, (&Consumer{Reporter: reporter}).ConsumeTraces(context.Background(), traces))
	return events
}

func newTracesSpans() (pdata.Traces, pdata.InstrumentationLibrarySpans) {
	traces := pdata.NewTraces()
	resourceSpans := pdata.NewResourceSpans()
	librarySpans := pdata.NewInstrumentationLibrarySpans()
	resourceSpans.InstrumentationLibrarySpans().Append(librarySpans)
	traces.ResourceSpans().Append(resourceSpans)
	return traces, librarySpans
}

func newString(s string) *string {
	return &s
}

func newInt(v int) *int {
	return &v
}
