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

package otlp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	jaegermodel "github.com/jaegertracing/jaeger/model"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modeljson"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestConsumer_ConsumeTraces_Interface(t *testing.T) {
	var _ consumer.Traces = otlp.NewConsumer(otlp.ConsumerConfig{})
}

func TestConsumer_ConsumeTraces_Empty(t *testing.T) {
	var processor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		assert.Empty(t, batch)
		return nil
	}

	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	traces := ptrace.NewTraces()
	result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeTracesResult{}, result)
}

func TestOutcome(t *testing.T) {
	test := func(t *testing.T, expectedOutcome, expectedResult string, statusCode ptrace.StatusCode) {
		t.Helper()

		traces, spans := newTracesSpans()
		otelSpan1 := spans.Spans().AppendEmpty()
		otelSpan1.SetTraceID(pcommon.TraceID{1})
		otelSpan1.SetSpanID(pcommon.SpanID{2})
		otelSpan1.Status().SetCode(statusCode)
		otelSpan2 := spans.Spans().AppendEmpty()
		otelSpan2.SetTraceID(pcommon.TraceID{1})
		otelSpan2.SetSpanID(pcommon.SpanID{2})
		otelSpan2.SetParentSpanID(pcommon.SpanID{3})
		otelSpan2.Status().SetCode(statusCode)

		batch := transformTraces(t, traces)
		require.Len(t, *batch, 2)

		assert.Equal(t, expectedOutcome, (*batch)[0].GetEvent().GetOutcome())
		assert.Equal(t, expectedResult, (*batch)[0].Transaction.Result)
		assert.Equal(t, expectedOutcome, (*batch)[1].GetEvent().GetOutcome())
	}

	test(t, "success", "Success", ptrace.StatusCodeUnset)
	test(t, "success", "Success", ptrace.StatusCodeOk)
	test(t, "failure", "Error", ptrace.StatusCodeError)
}

func TestRepresentativeCount(t *testing.T) {
	traces, spans := newTracesSpans()
	otelSpan1 := spans.Spans().AppendEmpty()
	otelSpan1.SetTraceID(pcommon.TraceID{1})
	otelSpan1.SetSpanID(pcommon.SpanID{2})
	otelSpan1.TraceState().FromRaw("esx:0.5,ot=p:8;r:62;k1:13,xy=w")
	otelSpan2 := spans.Spans().AppendEmpty()
	otelSpan2.SetTraceID(pcommon.TraceID{1})
	otelSpan2.SetSpanID(pcommon.SpanID{3})
	otelSpan2.SetParentSpanID(pcommon.SpanID{2})
	otelSpan2.TraceState().FromRaw("esx:0.5,ot=p:63;r:62;k1:13,xy=w")
	otelSpan3 := spans.Spans().AppendEmpty()
	otelSpan3.SetTraceID(pcommon.TraceID{1})
	otelSpan3.SetSpanID(pcommon.SpanID{4})
	otelSpan3.SetParentSpanID(pcommon.SpanID{2})
	otelSpan3.TraceState().FromRaw("esx:0.5,ot=p:0;r:62;k1:13,xy=w")
	otelSpan4 := spans.Spans().AppendEmpty()
	otelSpan4.SetTraceID(pcommon.TraceID{1})
	otelSpan4.SetSpanID(pcommon.SpanID{5})
	otelSpan4.SetParentSpanID(pcommon.SpanID{2})
	otelSpan4.TraceState().FromRaw("esx:0.5")
	otelSpan5 := spans.Spans().AppendEmpty()
	otelSpan5.SetTraceID(pcommon.TraceID{1})
	otelSpan5.SetSpanID(pcommon.SpanID{6})
	otelSpan5.SetParentSpanID(pcommon.SpanID{2})

	batch := transformTraces(t, traces)
	require.Len(t, *batch, 5)

	assert.Equal(t, 256.0, (*batch)[0].Transaction.RepresentativeCount)
	assert.Equal(t, 0.0, (*batch)[1].Span.RepresentativeCount)
	assert.Equal(t, 1.0, (*batch)[2].Span.RepresentativeCount)
	assert.Equal(t, 1.0, (*batch)[3].Span.RepresentativeCount)
	assert.Equal(t, 1.0, (*batch)[4].Span.RepresentativeCount)
}

func TestHTTPTransactionURL(t *testing.T) {
	test := func(t *testing.T, expected *modelpb.URL, attrs map[string]interface{}) {
		t.Helper()
		event := transformTransactionWithAttributes(t, attrs)
		assert.Equal(t, expected, event.Url)
	}

	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo?bar",
			Full:     "https://testing.invalid:80/foo?bar",
			Path:     "/foo",
			Query:    "bar",
			Domain:   "testing.invalid",
			Port:     80,
		}, map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "testing.invalid:80",
			"http.target": "/foo?bar",
		})
	})
	t.Run("scheme_servername_nethostport_target", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo?bar",
			Full:     "https://testing.invalid:80/foo?bar",
			Path:     "/foo",
			Query:    "bar",
			Domain:   "testing.invalid",
			Port:     80,
		}, map[string]interface{}{
			"http.scheme":      "https",
			"http.server_name": "testing.invalid",
			"net.host.port":    80,
			"http.target":      "/foo?bar",
		})
	})
	t.Run("scheme_nethostname_nethostport_target", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo?bar",
			Full:     "https://testing.invalid:80/foo?bar",
			Path:     "/foo",
			Query:    "bar",
			Domain:   "testing.invalid",
			Port:     80,
		}, map[string]interface{}{
			"http.scheme":   "https",
			"net.host.name": "testing.invalid",
			"net.host.port": 80,
			"http.target":   "/foo?bar",
		})
	})
	t.Run("http.url", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "https://testing.invalid:80/foo?bar",
			Full:     "https://testing.invalid:80/foo?bar",
			Path:     "/foo",
			Query:    "bar",
			Domain:   "testing.invalid",
			Port:     80,
		}, map[string]interface{}{
			"http.url": "https://testing.invalid:80/foo?bar",
		})
	})
	t.Run("host_no_port", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo",
			Full:     "https://testing.invalid/foo",
			Path:     "/foo",
			Domain:   "testing.invalid",
		}, map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "testing.invalid",
			"http.target": "/foo",
		})
	})
	t.Run("ipv6_host_no_port", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo",
			Full:     "https://[::1]/foo",
			Path:     "/foo",
			Domain:   "::1",
		}, map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "[::1]",
			"http.target": "/foo",
		})
	})
	t.Run("default_scheme", func(t *testing.T) {
		// scheme is set to "http" if it can't be deduced from attributes.
		test(t, &modelpb.URL{
			Scheme:   "http",
			Original: "/foo",
			Full:     "http://testing.invalid/foo",
			Path:     "/foo",
			Domain:   "testing.invalid",
		}, map[string]interface{}{
			"http.host":   "testing.invalid",
			"http.target": "/foo",
		})
	})
	t.Run("url_attributes", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo",
			Full:     "https://test.domain/foo",
			Path:     "/foo",
			Domain:   "test.domain",
		}, map[string]interface{}{
			"url.scheme":     "https",
			"server.address": "test.domain",
			"url.path":       "/foo",
		})
	})
	t.Run("url_attributes_with_query", func(t *testing.T) {
		test(t, &modelpb.URL{
			Scheme:   "https",
			Original: "/foo?bar",
			Full:     "https://test.domain/foo?bar",
			Path:     "/foo",
			Query:    "bar",
			Domain:   "test.domain",
		}, map[string]interface{}{
			"url.scheme":     "https",
			"server.address": "test.domain",
			"url.path":       "/foo",
			"url.query":      "bar",
		})
	})
}

func TestHTTPSpanURL(t *testing.T) {
	test := func(t *testing.T, expected string, attrs map[string]interface{}) {
		t.Helper()
		event := transformSpanWithAttributes(t, attrs)
		assert.Equal(t, &modelpb.URL{Original: expected}, event.Url)
	}

	t.Run("host.url", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]interface{}{
			"http.url": "https://testing.invalid:80/foo?bar",
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "testing.invalid:80",
			"http.target": "/foo?bar",
		})
	})
	t.Run("scheme_netpeername_netpeerport_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]interface{}{
			"http.scheme":   "https",
			"net.peer.name": "testing.invalid",
			"net.peer.ip":   "::1", // net.peer.name preferred
			"net.peer.port": 80,
			"http.target":   "/foo?bar",
		})
	})
	t.Run("scheme_netpeerip_netpeerport_target", func(t *testing.T) {
		test(t, "https://[::1]:80/foo?bar", map[string]interface{}{
			"http.scheme":   "https",
			"net.peer.ip":   "::1",
			"net.peer.port": 80,
			"http.target":   "/foo?bar",
		})
	})
	t.Run("default_scheme", func(t *testing.T) {
		// scheme is set to "http" if it can't be deduced from attributes.
		test(t, "http://testing.invalid/foo", map[string]interface{}{
			"http.host":   "testing.invalid",
			"http.target": "/foo",
		})
	})
}

func TestHTTPSpanDestination(t *testing.T) {
	test := func(t *testing.T, expectedDestination *modelpb.Destination, expectedDestinationService *modelpb.DestinationService, attrs map[string]interface{}) {
		t.Helper()
		event := transformSpanWithAttributes(t, attrs)
		assert.Empty(t, cmp.Diff(expectedDestination, event.Destination, protocmp.Transform()))
		assert.Empty(t, cmp.Diff(expectedDestinationService, event.Span.DestinationService, protocmp.Transform()))
	}

	t.Run("url_default_port_specified", func(t *testing.T) {
		test(t, &modelpb.Destination{
			Address: "testing.invalid",
			Port:    443,
		}, &modelpb.DestinationService{
			Type:     "external",
			Name:     "https://testing.invalid",
			Resource: "testing.invalid:443",
		}, map[string]interface{}{
			"http.url": "https://testing.invalid:443/foo?bar",
		})
	})
	t.Run("url_port_scheme", func(t *testing.T) {
		test(t, &modelpb.Destination{
			Address: "testing.invalid",
			Port:    443,
		}, &modelpb.DestinationService{
			Type:     "external",
			Name:     "https://testing.invalid",
			Resource: "testing.invalid:443",
		}, map[string]interface{}{
			"http.url": "https://testing.invalid/foo?bar",
		})
	})
	t.Run("url_non_default_port", func(t *testing.T) {
		test(t, &modelpb.Destination{
			Address: "testing.invalid",
			Port:    444,
		}, &modelpb.DestinationService{
			Type:     "external",
			Name:     "https://testing.invalid:444",
			Resource: "testing.invalid:444",
		}, map[string]interface{}{
			"http.url": "https://testing.invalid:444/foo?bar",
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, &modelpb.Destination{
			Address: "testing.invalid",
			Port:    444,
		}, &modelpb.DestinationService{
			Type:     "external",
			Name:     "https://testing.invalid:444",
			Resource: "testing.invalid:444",
		}, map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "testing.invalid:444",
			"http.target": "/foo?bar",
		})
	})
	t.Run("scheme_netpeername_nethostport_target", func(t *testing.T) {
		test(t, &modelpb.Destination{
			Address: "::1",
			Port:    444,
		}, &modelpb.DestinationService{
			Type:     "external",
			Name:     "https://[::1]:444",
			Resource: "[::1]:444",
		}, map[string]interface{}{
			"http.scheme":   "https",
			"net.peer.ip":   "::1",
			"net.peer.port": 444,
			"http.target":   "/foo?bar",
		})
	})
}

func TestHTTPTransactionSource(t *testing.T) {
	test := func(t *testing.T, expectedDomain, expectedIP string, expectedPort int, attrs map[string]interface{}) {
		// "http.method" is a required attribute for HTTP spans,
		// and its presence causes the transaction's HTTP request
		// context to be built.
		attrs["http.method"] = "POST"

		event := transformTransactionWithAttributes(t, attrs)
		require.NotNil(t, event.Http)
		require.NotNil(t, event.Http.Request)
		parsedIP, err := modelpb.ParseIP(expectedIP)
		require.NoError(t, err)
		assert.Equal(t, &modelpb.Source{
			Domain: expectedDomain,
			Ip:     parsedIP,
			Port:   uint32(expectedPort),
		}, event.Source)
		want := modelpb.Client{Ip: event.Source.Ip, Port: event.Source.Port, Domain: event.Source.Domain}
		assert.Equal(t, &want, event.Client)
	}

	t.Run("net.peer.ip_port", func(t *testing.T) {
		test(t, "", "192.168.0.1", 1234, map[string]interface{}{
			"net.peer.ip":   "192.168.0.1",
			"net.peer.port": 1234,
		})
	})
	t.Run("net.peer.ip", func(t *testing.T) {
		test(t, "", "192.168.0.1", 0, map[string]interface{}{
			"net.peer.ip": "192.168.0.1",
		})
	})
	t.Run("net.peer.ip_name", func(t *testing.T) {
		test(t, "source.domain", "192.168.0.1", 0, map[string]interface{}{
			"net.peer.name": "source.domain",
			"net.peer.ip":   "192.168.0.1",
		})
	})
}

func TestHTTPTransactionFlavor(t *testing.T) {
	event := transformTransactionWithAttributes(t, map[string]interface{}{
		"http.flavor": "1.1",
	})
	assert.Equal(t, "1.1", event.Http.Version)
}

func TestHTTPTransactionUserAgent(t *testing.T) {
	test := func(t *testing.T, attrs map[string]interface{}) {
		t.Helper()
		event := transformTransactionWithAttributes(t, attrs)
		assert.Equal(t, &modelpb.UserAgent{Original: "Foo/bar (baz)"}, event.UserAgent)
	}

	t.Run("http.user_agent", func(t *testing.T) {
		test(t, map[string]interface{}{
			"http.user_agent": "Foo/bar (baz)",
		})
	})

	t.Run("user_agent.original", func(t *testing.T) {
		test(t, map[string]interface{}{
			"user_agent.original": "Foo/bar (baz)",
		})
	})
}

func TestHTTPTransactionClientIP(t *testing.T) {
	event := transformTransactionWithAttributes(t, map[string]interface{}{
		"net.peer.ip":    "1.2.3.4",
		"net.peer.port":  5678,
		"http.client_ip": "9.10.11.12",
	})
	ip, _ := modelpb.ParseIP("1.2.3.4")
	ip2, _ := modelpb.ParseIP("9.10.11.12")
	assert.Equal(t, &modelpb.Source{Ip: ip, Port: 5678}, event.Source)
	assert.Equal(t, &modelpb.Client{Ip: ip2}, event.Client)
}

func TestHTTPTransactionStatusCode(t *testing.T) {
	test := func(t *testing.T, expected uint32, attrs map[string]interface{}) {
		t.Helper()
		event := transformSpanWithAttributes(t, attrs)
		assert.Equal(t, expected, event.Http.Response.StatusCode)
	}

	t.Run("http.status_code", func(t *testing.T) {
		test(t, 200, map[string]interface{}{
			"http.status_code": 200,
		})
	})

	t.Run("http.response.status_code", func(t *testing.T) {
		test(t, 200, map[string]interface{}{
			"http.response.status_code": 200,
		})
	})
}

func TestDatabaseSpan(t *testing.T) {
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/database.md#mysql
	connectionString := "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;"
	dbStatement := fmt.Sprintf("SELECT * FROM orders WHERE order_id = '%s'", strings.Repeat("*", 1024)) // should not be truncated!
	event := transformSpanWithAttributes(t, map[string]interface{}{
		"db.system":            "mysql",
		"db.connection_string": connectionString,
		"db.user":              "billing_user",
		"db.name":              "ShopDb",
		"db.statement":         dbStatement,
		"net.peer.name":        "shopdb.example.com",
		"net.peer.ip":          "192.0.2.12",
		"net.peer.port":        3306,
		"net.transport":        "IP.TCP",
	})

	assert.Equal(t, "db", event.Span.Type)
	assert.Equal(t, "mysql", event.Span.Subtype)
	assert.Equal(t, "", event.Span.Action)

	assert.Equal(t, &modelpb.DB{
		Instance:  "ShopDb",
		Statement: dbStatement,
		Type:      "mysql",
		UserName:  "billing_user",
	}, event.Span.Db)

	assert.Equal(t, modelpb.Labels{
		"db_connection_string": {Value: connectionString},
		"net_transport":        {Value: "IP.TCP"},
	}, modelpb.Labels(event.Labels))

	assert.Equal(t, &modelpb.Destination{
		Address: "shopdb.example.com",
		Port:    3306,
	}, event.Destination)

	assert.Empty(t, cmp.Diff(&modelpb.DestinationService{
		Type:     "db",
		Name:     "mysql",
		Resource: "mysql",
	}, event.Span.DestinationService, protocmp.Transform()))
}

func TestDatabaseSpanWithServerAttributes(t *testing.T) {
	event := transformSpanWithAttributes(t, map[string]interface{}{
		"db.system":      "mysql",
		"db.name":        "ShopDb",
		"server.address": "shopdb.example.com",
		"server.port":    3306,
	})

	assert.Equal(t, "db", event.Span.Type)
	assert.Equal(t, "mysql", event.Span.Subtype)
	assert.Equal(t, "", event.Span.Action)

	assert.Equal(t, &modelpb.Destination{
		Address: "shopdb.example.com",
		Port:    3306,
	}, event.Destination)
}

func TestInstrumentationLibrary(t *testing.T) {
	traces, spans := newTracesSpans()
	spans.Scope().SetName("library-name")
	spans.Scope().SetVersion("1.2.3")
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	events := transformTraces(t, traces)
	event := (*events)[0]

	assert.Equal(t, "library-name", event.Service.Framework.Name)
	assert.Equal(t, "1.2.3", event.Service.Framework.Version)
}

func TestRPCTransaction(t *testing.T) {
	event := transformTransactionWithAttributes(t, map[string]interface{}{
		"rpc.system":           "grpc",
		"rpc.service":          "myservice.EchoService",
		"rpc.method":           "exampleMethod",
		"rpc.grpc.status_code": int64(codes.Unavailable),
		"net.peer.name":        "peer_name",
		"net.peer.ip":          "10.20.30.40",
		"net.peer.port":        123,
	})
	assert.Equal(t, "request", event.Transaction.Type)
	assert.Equal(t, "Unavailable", event.Transaction.Result)
	assert.Empty(t, event.Labels)

	ip, _ := modelpb.ParseIP("10.20.30.40")
	assert.Equal(t, &modelpb.Client{
		Domain: "peer_name",
		Ip:     ip,
		Port:   123,
	}, event.Client)
}

func TestRPCSpan(t *testing.T) {
	event := transformSpanWithAttributes(t, map[string]interface{}{
		"rpc.system":           "grpc",
		"rpc.service":          "myservice.EchoService",
		"rpc.method":           "exampleMethod",
		"rpc.grpc.status_code": int64(codes.Unavailable),
		"net.peer.ip":          "10.20.30.40",
		"net.peer.port":        123,
	})
	assert.Equal(t, "external", event.Span.Type)
	assert.Equal(t, "grpc", event.Span.Subtype)
	assert.Empty(t, event.Labels)
	assert.Equal(t, &modelpb.Destination{
		Address: "10.20.30.40",
		Port:    123,
	}, event.Destination)
	assert.Empty(t, cmp.Diff(&modelpb.DestinationService{
		Type:     "external",
		Name:     "10.20.30.40:123",
		Resource: "10.20.30.40:123",
	}, event.Span.DestinationService, protocmp.Transform()))
}

func TestMessagingTransaction(t *testing.T) {
	for _, tc := range []struct {
		attrs map[string]interface{}

		expectedLabels     map[string]*modelpb.LabelValue
		expectedTxnMessage *modelpb.Message
	}{
		{
			attrs: map[string]interface{}{
				"messaging.destination": "myQueue",
			},
			expectedLabels: nil,
			expectedTxnMessage: &modelpb.Message{
				QueueName: "myQueue",
			},
		},
		{
			attrs: map[string]interface{}{
				"messaging.system": "kafka",
			},
			expectedLabels: map[string]*modelpb.LabelValue{
				"messaging_system": {Value: "kafka"},
			},
			expectedTxnMessage: nil,
		},
		{
			attrs: map[string]interface{}{
				"messaging.operation": "publish",
			},
			expectedLabels: map[string]*modelpb.LabelValue{
				"messaging_operation": {Value: "publish"},
			},
			expectedTxnMessage: nil,
		},
	} {
		tcName, err := json.Marshal(tc.attrs)
		require.NoError(t, err)
		t.Run(string(tcName), func(t *testing.T) {
			event := transformTransactionWithAttributes(t, tc.attrs, func(s ptrace.Span) {
				s.SetKind(ptrace.SpanKindConsumer)
				// Set parentID to imply this isn't the root, but
				// kind==Consumer should still force the span to be translated
				// as a transaction.
				s.SetParentSpanID(pcommon.SpanID{3})
			})
			assert.Equal(t, "messaging", event.Transaction.Type)
			assert.Equal(t, tc.expectedLabels, event.Labels)
			assert.Equal(t, tc.expectedTxnMessage, event.Transaction.Message)
		})
	}

}

func TestMessagingSpan(t *testing.T) {
	for _, attr := range []map[string]any{
		{
			"messaging.system":      "kafka",
			"messaging.destination": "myTopic",
			"net.peer.ip":           "10.20.30.40",
			"net.peer.port":         123,
		},
		{
			"messaging.system":           "kafka",
			"messaging.destination.name": "myTopic",
			"net.peer.ip":                "10.20.30.40",
			"net.peer.port":              123,
		},
	} {
		event := transformSpanWithAttributes(t, attr, func(s ptrace.Span) {
			s.SetKind(ptrace.SpanKindProducer)
		})
		assert.Equal(t, "messaging", event.Span.Type)
		assert.Equal(t, "kafka", event.Span.Subtype)
		assert.Equal(t, "send", event.Span.Action)
		assert.Empty(t, event.Labels)
		assert.Equal(t, &modelpb.Destination{
			Address: "10.20.30.40",
			Port:    123,
		}, event.Destination)
		assert.Empty(t, cmp.Diff(&modelpb.DestinationService{
			Type:     "messaging",
			Name:     "kafka",
			Resource: "kafka/myTopic",
		}, event.Span.DestinationService, protocmp.Transform()))
	}
}

func TestMessagingSpan_DestinationResource(t *testing.T) {
	test := func(t *testing.T, expectedDestination *modelpb.Destination, expectedDestinationService *modelpb.DestinationService, attrs map[string]interface{}) {
		t.Helper()
		event := transformSpanWithAttributes(t, attrs)
		assert.Empty(t, cmp.Diff(expectedDestination, event.Destination, protocmp.Transform()))
		assert.Empty(t, cmp.Diff(expectedDestinationService, event.Span.DestinationService, protocmp.Transform()))
	}

	setAttr := func(t *testing.T, baseAttr map[string]any, key string, val any) map[string]any {
		t.Helper()
		newAttr := make(map[string]any)
		// Copy from the original map to the target map
		for key, value := range baseAttr {
			newAttr[key] = value
		}
		newAttr[key] = val
		return newAttr
	}

	t.Run("system_destination_peerservice_peeraddress", func(t *testing.T) {
		baseAttr := map[string]any{
			"messaging.system": "kafka",
			"peer.service":     "testsvc",
			"peer.address":     "127.0.0.1",
		}
		for _, attr := range []map[string]any{
			setAttr(t, baseAttr, "messaging.destination", "testtopic"),
			setAttr(t, baseAttr, "messaging.destination.name", "testtopic"),
		} {
			test(t, &modelpb.Destination{
				Address: "127.0.0.1",
			}, &modelpb.DestinationService{
				Type:     "messaging",
				Name:     "testsvc",
				Resource: "127.0.0.1/testtopic",
			}, attr)
		}
	})
	t.Run("system_destination_peerservice", func(t *testing.T) {
		baseAttr := map[string]any{
			"messaging.system": "kafka",
			"peer.service":     "testsvc",
		}
		for _, attr := range []map[string]any{
			setAttr(t, baseAttr, "messaging.destination", "testtopic"),
			setAttr(t, baseAttr, "messaging.destination.name", "testtopic"),
		} {
			test(t, nil, &modelpb.DestinationService{
				Type:     "messaging",
				Name:     "testsvc",
				Resource: "testsvc/testtopic",
			}, attr)
		}
	})
	t.Run("system_destination", func(t *testing.T) {
		baseAttr := map[string]any{
			"messaging.system": "kafka",
		}
		for _, attr := range []map[string]any{
			setAttr(t, baseAttr, "messaging.destination", "testtopic"),
			setAttr(t, baseAttr, "messaging.destination.name", "testtopic"),
		} {
			test(t, nil, &modelpb.DestinationService{
				Type:     "messaging",
				Name:     "kafka",
				Resource: "kafka/testtopic",
			}, attr)
		}
	})
}

func TestSpanType(t *testing.T) {
	// Internal spans default to app.internal.
	event := transformSpanWithAttributes(t, map[string]interface{}{}, func(s ptrace.Span) {
		s.SetKind(ptrace.SpanKindInternal)
	})
	assert.Equal(t, "app", event.Span.Type)
	assert.Equal(t, "internal", event.Span.Subtype)

	// All other spans default to unknown.
	event = transformSpanWithAttributes(t, map[string]interface{}{}, func(s ptrace.Span) {
		s.SetKind(ptrace.SpanKindClient)
	})
	assert.Equal(t, "unknown", event.Span.Type)
	assert.Equal(t, "", event.Span.Subtype)
}

func TestTransactionTypePriorities(t *testing.T) {

	transactionWithAttribs := func(attributes map[string]interface{}) *modelpb.APMEvent {
		return transformTransactionWithAttributes(t, attributes, func(s ptrace.Span) {
			s.SetKind(ptrace.SpanKindServer)
		})
	}

	attribs := map[string]interface{}{
		"http.scheme": "https",
	}
	assert.Equal(t, "request", transactionWithAttribs(attribs).Transaction.Type)

	attribs["messaging.destination"] = "foobar"
	assert.Equal(t, "messaging", transactionWithAttribs(attribs).Transaction.Type)
	delete(attribs, "messaging.destination")
	attribs["messaging.destination.name"] = "foobar"
	assert.Equal(t, "messaging", transactionWithAttribs(attribs).Transaction.Type)
}

func TestSpanTypePriorities(t *testing.T) {

	spanWithAttribs := func(attributes map[string]interface{}) *modelpb.APMEvent {
		return transformSpanWithAttributes(t, attributes, func(s ptrace.Span) {
			s.SetKind(ptrace.SpanKindClient)
		})
	}

	attribs := map[string]interface{}{
		"http.scheme": "https",
	}
	assert.Equal(t, "http", spanWithAttribs(attribs).Span.Subtype)

	attribs["rpc.grpc.status_code"] = int64(codes.Unavailable)
	assert.Equal(t, "grpc", spanWithAttribs(attribs).Span.Subtype)

	attribs["messaging.destination"] = "foobar"
	assert.Equal(t, "messaging", spanWithAttribs(attribs).Span.Type)

	delete(attribs, "messaging.destination")
	attribs["messaging.destination.name"] = "foobar"
	assert.Equal(t, "messaging", spanWithAttribs(attribs).Span.Type)

	attribs["db.statement"] = "SELECT * FROM FOO"
	assert.Equal(t, "db", spanWithAttribs(attribs).Span.Type)
}

func TestSpanNetworkAttributes(t *testing.T) {
	networkAttributes := map[string]interface{}{
		"network.connection.type":    "cell",
		"network.connection.subtype": "LTE",
		"network.carrier.name":       "Vodafone",
		"network.carrier.mnc":        "01",
		"network.carrier.mcc":        "101",
		"network.carrier.icc":        "UK",
	}
	txEvent := transformTransactionWithAttributes(t, networkAttributes)
	spanEvent := transformSpanWithAttributes(t, networkAttributes)

	expected := modelpb.Network{
		Connection: &modelpb.NetworkConnection{
			Type:    "cell",
			Subtype: "LTE",
		},
		Carrier: &modelpb.NetworkCarrier{
			Name: "Vodafone",
			Mnc:  "01",
			Mcc:  "101",
			Icc:  "UK",
		},
	}
	assert.Equal(t, &expected, txEvent.Network)
	assert.Equal(t, &expected, spanEvent.Network)
}

func TestSpanDataStream(t *testing.T) {
	for _, tc := range []struct {
		resourceDataStreamDataset   string
		resourceDataStreamNamespace string
		scopeDataStreamDataset      string
		scopeDataStreamNamespace    string
		recordDataStreamDataset     string
		recordDataStreamNamespace   string

		expectedDataStreamDataset   string
		expectedDataStreamNamespace string
	}{
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			scopeDataStreamDataset:      "3",
			scopeDataStreamNamespace:    "4",
			recordDataStreamDataset:     "5",
			recordDataStreamNamespace:   "6",
			expectedDataStreamDataset:   "5",
			expectedDataStreamNamespace: "6",
		},
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			scopeDataStreamDataset:      "3",
			scopeDataStreamNamespace:    "4",
			expectedDataStreamDataset:   "3",
			expectedDataStreamNamespace: "4",
		},
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			expectedDataStreamDataset:   "1",
			expectedDataStreamNamespace: "2",
		},
	} {
		for _, isTxn := range []bool{false, true} {
			tcName := fmt.Sprintf("%s,%s,txn=%v", tc.expectedDataStreamDataset, tc.expectedDataStreamNamespace, isTxn)
			t.Run(tcName, func(t *testing.T) {
				traces := ptrace.NewTraces()
				resourceSpans := traces.ResourceSpans().AppendEmpty()
				resourceAttrs := traces.ResourceSpans().At(0).Resource().Attributes()
				if tc.resourceDataStreamDataset != "" {
					resourceAttrs.PutStr("data_stream.dataset", tc.resourceDataStreamDataset)
				}
				if tc.resourceDataStreamNamespace != "" {
					resourceAttrs.PutStr("data_stream.namespace", tc.resourceDataStreamNamespace)
				}

				scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
				scopeAttrs := resourceSpans.ScopeSpans().At(0).Scope().Attributes()
				if tc.scopeDataStreamDataset != "" {
					scopeAttrs.PutStr("data_stream.dataset", tc.scopeDataStreamDataset)
				}
				if tc.scopeDataStreamNamespace != "" {
					scopeAttrs.PutStr("data_stream.namespace", tc.scopeDataStreamNamespace)
				}

				otelSpan := scopeSpans.Spans().AppendEmpty()
				otelSpan.SetTraceID(pcommon.TraceID{1})
				otelSpan.SetSpanID(pcommon.SpanID{2})
				if !isTxn {
					otelSpan.SetParentSpanID(pcommon.SpanID{3})
				}
				if tc.recordDataStreamDataset != "" {
					otelSpan.Attributes().PutStr("data_stream.dataset", tc.recordDataStreamDataset)
				}
				if tc.recordDataStreamNamespace != "" {
					otelSpan.Attributes().PutStr("data_stream.namespace", tc.recordDataStreamNamespace)
				}
				events := transformTraces(t, traces)

				dataStream := &modelpb.DataStream{
					Dataset:   tc.expectedDataStreamDataset,
					Namespace: tc.expectedDataStreamNamespace,
				}

				assert.Equal(t, dataStream, (*events)[0].DataStream)
			})
		}
	}
}

func TestSessionID(t *testing.T) {
	sessionAttributes := map[string]interface{}{
		"session.id": "opbeans-swift",
	}
	txEvent := transformTransactionWithAttributes(t, sessionAttributes)
	spanEvent := transformSpanWithAttributes(t, sessionAttributes)

	expected := modelpb.Session{
		Id: "opbeans-swift",
	}
	assert.Equal(t, &expected, txEvent.Session)
	assert.Equal(t, &expected, spanEvent.Session)
}

func TestArrayLabels(t *testing.T) {
	stringArray := []interface{}{"string1", "string2"}
	boolArray := []interface{}{false, true}
	intArray := []interface{}{1234, 5678}
	floatArray := []interface{}{1234.5678, 9123.234123123}

	txEvent := transformTransactionWithAttributes(t, map[string]interface{}{
		"string_array": stringArray,
		"bool_array":   boolArray,
		"int_array":    intArray,
		"float_array":  floatArray,
	})
	assert.Equal(t, modelpb.Labels{
		"bool_array":   {Values: []string{"false", "true"}},
		"string_array": {Values: []string{"string1", "string2"}},
	}, modelpb.Labels(txEvent.Labels))
	assert.Equal(t, modelpb.NumericLabels{
		"int_array":   {Values: []float64{1234, 5678}},
		"float_array": {Values: []float64{1234.5678, 9123.234123123}},
	}, modelpb.NumericLabels(txEvent.NumericLabels))

	spanEvent := transformSpanWithAttributes(t, map[string]interface{}{
		"string_array": stringArray,
		"bool_array":   boolArray,
		"int_array":    intArray,
		"float_array":  floatArray,
	})
	assert.Equal(t, modelpb.Labels{
		"bool_array":   {Values: []string{"false", "true"}},
		"string_array": {Values: []string{"string1", "string2"}},
	}, modelpb.Labels(spanEvent.Labels))
	assert.Equal(t, modelpb.NumericLabels{
		"int_array":   {Values: []float64{1234, 5678}},
		"float_array": {Values: []float64{1234.5678, 9123.234123123}},
	}, modelpb.NumericLabels(spanEvent.NumericLabels))
}

func TestProfilerStackTraceIds(t *testing.T) {
	validIds := []interface{}{"myId1", "myId2"}
	badValueTypes := []interface{}{42, 68, "valid"}

	tx1 := transformTransactionWithAttributes(t, map[string]interface{}{
		"elastic.profiler_stack_trace_ids": validIds,
	})
	assert.Equal(t, []string{"myId1", "myId2"}, tx1.Transaction.ProfilerStackTraceIds)

	tx2 := transformTransactionWithAttributes(t, map[string]interface{}{
		"elastic.profiler_stack_trace_ids": badValueTypes,
	})
	assert.Equal(t, []string{"valid"}, tx2.Transaction.ProfilerStackTraceIds)

	tx3 := transformTransactionWithAttributes(t, map[string]interface{}{
		"elastic.profiler_stack_trace_ids": "bad type",
	})
	assert.Equal(t, []string(nil), tx3.Transaction.ProfilerStackTraceIds)
}

func TestConsumeTracesExportTimestamp(t *testing.T) {
	traces, otelSpans := newTracesSpans()

	// The actual timestamps will be non-deterministic, as they are adjusted
	// based on the server's clock.
	//
	// Use a large delta so that we can allow for a significant amount of
	// delay in the test environment affecting the timestamp adjustment.
	const timeDelta = time.Hour
	const allowedError = 5 * time.Second // seconds

	now := time.Now()
	exportTimestamp := now.Add(-timeDelta)
	traces.ResourceSpans().At(0).Resource().Attributes().PutInt("telemetry.sdk.elastic_export_timestamp", exportTimestamp.UnixNano())

	// Offsets are start times relative to the export timestamp.
	transactionOffset := -2 * time.Second
	spanOffset := transactionOffset + time.Second
	exceptionOffset := spanOffset + 25*time.Millisecond
	transactionDuration := time.Second + 100*time.Millisecond
	spanDuration := 50 * time.Millisecond

	exportedTransactionTimestamp := exportTimestamp.Add(transactionOffset)
	exportedSpanTimestamp := exportTimestamp.Add(spanOffset)
	exportedExceptionTimestamp := exportTimestamp.Add(exceptionOffset)

	otelSpan1 := otelSpans.Spans().AppendEmpty()
	otelSpan1.SetTraceID(pcommon.TraceID{1})
	otelSpan1.SetSpanID(pcommon.SpanID{2})
	otelSpan1.SetStartTimestamp(pcommon.NewTimestampFromTime(exportedTransactionTimestamp))
	otelSpan1.SetEndTimestamp(pcommon.NewTimestampFromTime(exportedTransactionTimestamp.Add(transactionDuration)))

	otelSpan2 := otelSpans.Spans().AppendEmpty()
	otelSpan2.SetTraceID(pcommon.TraceID{1})
	otelSpan2.SetSpanID(pcommon.SpanID{2})
	otelSpan2.SetParentSpanID(pcommon.SpanID{3})
	otelSpan2.SetStartTimestamp(pcommon.NewTimestampFromTime(exportedSpanTimestamp))
	otelSpan2.SetEndTimestamp(pcommon.NewTimestampFromTime(exportedSpanTimestamp.Add(spanDuration)))

	otelSpanEvent := otelSpan2.Events().AppendEmpty()
	otelSpanEvent.SetTimestamp(pcommon.NewTimestampFromTime(exportedExceptionTimestamp))
	otelSpanEvent.SetName("exception")
	otelSpanEvent.Attributes().PutStr("exception.type", "the_type")
	otelSpanEvent.Attributes().PutStr("exception.message", "the_message")
	otelSpanEvent.Attributes().PutStr("exception.stacktrace", "the_stacktrace")

	batch := transformTraces(t, traces)
	require.Len(t, *batch, 3)

	// Give some leeway for one event, and check other events' timestamps relative to that one.
	assert.InDelta(t, modelpb.FromTime(now.Add(transactionOffset)), (*batch)[0].Timestamp, float64(allowedError.Nanoseconds()))
	assert.Equal(t, uint64((spanOffset - transactionOffset).Nanoseconds()), (*batch)[1].Timestamp-(*batch)[0].Timestamp)
	assert.Equal(t, uint64((exceptionOffset - transactionOffset).Nanoseconds()), (*batch)[2].Timestamp-(*batch)[0].Timestamp)

	// Durations should be unaffected.
	assert.Equal(t, transactionDuration, time.Duration((*batch)[0].GetEvent().GetDuration()))
	assert.Equal(t, spanDuration, time.Duration((*batch)[1].GetEvent().GetDuration()))

	for _, b := range *batch {
		// telemetry.sdk.elastic_export_timestamp should not be sent as a label.
		assert.Empty(t, b.NumericLabels)
	}
}

func TestConsumeTracesEventReceived(t *testing.T) {
	traces, otelSpans := newTracesSpans()
	now := modelpb.FromTime(time.Now())

	otelSpan1 := otelSpans.Spans().AppendEmpty()
	otelSpan1.SetTraceID(pcommon.TraceID{1})
	otelSpan1.SetSpanID(pcommon.SpanID{2})

	batch := transformTraces(t, traces)
	require.Len(t, *batch, 1)

	const allowedDelta = 2 * time.Second // seconds
	require.InDelta(t, now, (*batch)[0].Event.Received, float64(allowedDelta.Nanoseconds()))
}

func TestSpanLinks(t *testing.T) {
	linkedTraceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	linkedSpanID := pcommon.SpanID{7, 6, 5, 4, 3, 2, 1, 0}
	spanLink := ptrace.NewSpanLink()
	spanLink.SetSpanID(linkedSpanID)
	spanLink.SetTraceID(linkedTraceID)

	childSpanID1 := pcommon.SpanID{16, 17, 18, 19, 20, 21, 22, 23}
	childLink1 := ptrace.NewSpanLink()
	childLink1.SetSpanID(childSpanID1)
	childLink1.SetTraceID(linkedTraceID)
	childLink1.Attributes().PutBool("elastic.is_child", true)

	childSpanID2 := pcommon.SpanID{24, 25, 26, 27, 28, 29, 30, 31}
	childLink2 := ptrace.NewSpanLink()
	childLink2.SetSpanID(childSpanID2)
	childLink2.SetTraceID(linkedTraceID)
	childLink2.Attributes().PutBool("is_child", true)

	txEvent := transformTransactionWithAttributes(t, map[string]interface{}{}, func(span ptrace.Span) {
		spanLink.CopyTo(span.Links().AppendEmpty())
	})
	spanEvent := transformSpanWithAttributes(t, map[string]interface{}{}, func(span ptrace.Span) {
		spanLink.CopyTo(span.Links().AppendEmpty())
		childLink1.CopyTo(span.Links().AppendEmpty())
		childLink2.CopyTo(span.Links().AppendEmpty())
	})
	for _, event := range []*modelpb.APMEvent{txEvent, spanEvent} {
		assert.Equal(t, []*modelpb.SpanLink{{
			SpanId:  "0706050403020100",
			TraceId: "000102030405060708090a0b0c0d0e0f",
		}}, event.Span.Links)
	}
	assert.Equal(t, spanEvent.ChildIds, []string{"1011121314151617", "18191a1b1c1d1e1f"})
}

func TestConsumeTracesSemaphore(t *testing.T) {
	traces := ptrace.NewTraces()
	var batches []*modelpb.Batch

	doneCh := make(chan struct{})
	recorder := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		<-doneCh
		batchCopy := batch.Clone()
		batches = append(batches, &batchCopy)
		return nil
	})
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: recorder,
		Semaphore: semaphore.NewWeighted(1),
	})

	startCh := make(chan struct{})
	go func() {
		close(startCh)
		_, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
		assert.NoError(t, err)
	}()

	<-startCh
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := consumer.ConsumeTracesWithResult(ctx, traces)
	assert.Equal(t, err.Error(), "context deadline exceeded")
	close(doneCh)

	_, err = consumer.ConsumeTracesWithResult(context.Background(), traces)
	assert.NoError(t, err)
}

func TestConsumer_JaegerMetadata(t *testing.T) {
	jaegerBatch := &jaegermodel.Batch{
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
			var batches []*modelpb.Batch
			recorder := batchRecorderBatchProcessor(&batches)
			consumer := otlp.NewConsumer(otlp.ConsumerConfig{
				Processor: recorder,
				Semaphore: semaphore.NewWeighted(1),
			})

			jaegerBatch.Process = tc.process
			traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{jaegerBatch})
			require.NoError(t, err)
			result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
			require.NoError(t, err)
			require.Equal(t, otlp.ConsumeTracesResult{}, result)

			docs := encodeBatch(t, batches...)
			approveEventDocs(t, "metadata_"+tc.name, docs)
		})
	}
}

func TestConsumer_JaegerSampleRate(t *testing.T) {
	traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{{
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
				jaegerKeyValue("sampler.type", "const"),
				jaegerKeyValue("sampler.param", 1.0),
			},
		}},
	}})
	require.NoError(t, err)

	var batches []*modelpb.Batch
	recorder := batchRecorderBatchProcessor(&batches)
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: recorder,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
	require.NoError(t, err)
	require.Equal(t, otlp.ConsumeTracesResult{}, result)
	require.Len(t, batches, 1)
	batch := *batches[0]

	docs := encodeBatch(t, batches...)
	approveEventDocs(t, "jaeger_sampling_rate", docs)

	tx1 := batch[0].Transaction
	span := batch[1].Span
	tx2 := batch[2].Transaction
	assert.Equal(t, 1.25 /* 1/0.8 */, tx1.RepresentativeCount)
	assert.Equal(t, 2.5 /* 1/0.4 */, span.RepresentativeCount)
	assert.Zero(t, tx2.RepresentativeCount) // not set for non-probabilistic
}

func TestConsumer_JaegerTraceID(t *testing.T) {
	var batches []*modelpb.Batch
	recorder := batchRecorderBatchProcessor(&batches)
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: recorder,
		Semaphore: semaphore.NewWeighted(100),
	})

	traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{{
		Process: jaegermodel.NewProcess("", jaegerKeyValues("jaeger.version", "unknown")),
		Spans: []*jaegermodel.Span{{
			TraceID: jaegermodel.NewTraceID(0, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(456),
		}, {
			TraceID: jaegermodel.NewTraceID(0x000046467830, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(789),
		}},
	}})
	require.NoError(t, err)
	result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
	require.NoError(t, err)
	require.Equal(t, otlp.ConsumeTracesResult{}, result)

	batch := *batches[0]
	assert.Equal(t, "00000000000000000000000046467830", batch[0].Trace.Id)
	assert.Equal(t, "00000000464678300000000046467830", batch[1].Trace.Id)
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
				StartTime: testStartTime(),
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("component", "amqp"),
				},
			}},
		},
		{
			name: "jaeger_custom",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
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
				},
			}},
		},
		{
			name: "jaeger_data_stream",
			spans: []*jaegermodel.Span{{
				StartTime: testStartTime(),
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("data_stream.dataset", "1"),
					jaegerKeyValue("data_stream.namespace", "2"),
				},
			}},
		},
		{
			name: "jaeger_data_stream_with_error",
			spans: []*jaegermodel.Span{{
				TraceID: jaegermodel.NewTraceID(0, 0x46467830),
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("data_stream.dataset", "1"),
					jaegerKeyValue("data_stream.namespace", "2"),
				},
				Logs: []jaegermodel.Log{{
					Timestamp: testStartTime().Add(23 * time.Nanosecond),
					Fields: jaegerKeyValues(
						"event", "retrying connection",
						"level", "error",
						"error", "no connection established",
						"data_stream.dataset", "3",
						"data_stream.namespace", "4",
					),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{{
				Process: jaegermodel.NewProcess("", []jaegermodel.KeyValue{
					jaegerKeyValue("hostname", "host-abc"),
					jaegerKeyValue("jaeger.version", "unknown"),
				}),
				Spans: tc.spans,
			}})
			require.NoError(t, err)

			var batches []*modelpb.Batch
			recorder := batchRecorderBatchProcessor(&batches)
			consumer := otlp.NewConsumer(otlp.ConsumerConfig{
				Processor: recorder,
				Semaphore: semaphore.NewWeighted(100),
			})
			result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
			require.NoError(t, err)
			require.Equal(t, otlp.ConsumeTracesResult{}, result)

			docs := encodeBatch(t, batches...)
			approveEventDocs(t, "transaction_"+tc.name, docs)
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
		{
			name: "jaeger_data_stream",
			spans: []*jaegermodel.Span{{
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("data_stream.dataset", "1"),
					jaegerKeyValue("data_stream.namespace", "2"),
				},
			}},
		},
		{
			name: "jaeger_data_stream_with_error",
			spans: []*jaegermodel.Span{{
				Tags: []jaegermodel.KeyValue{
					jaegerKeyValue("data_stream.dataset", "1"),
					jaegerKeyValue("data_stream.namespace", "2"),
				},
				Logs: []jaegermodel.Log{{
					Timestamp: testStartTime().Add(23 * time.Nanosecond),
					Fields: jaegerKeyValues(
						"event", "retrying connection",
						"level", "error",
						"error", "no connection established",
						"data_stream.dataset", "3",
						"data_stream.namespace", "4",
					),
				}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := &jaegermodel.Batch{
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
			traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{batch})
			require.NoError(t, err)

			var batches []*modelpb.Batch
			recorder := batchRecorderBatchProcessor(&batches)
			consumer := otlp.NewConsumer(otlp.ConsumerConfig{
				Processor: recorder,
				Semaphore: semaphore.NewWeighted(100),
			})
			result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
			require.NoError(t, err)
			require.Equal(t, otlp.ConsumeTracesResult{}, result)

			docs := encodeBatch(t, batches...)
			approveEventDocs(t, "span_"+tc.name, docs)
		})
	}
}

func TestJaegerServiceVersion(t *testing.T) {
	traces, err := jaegertranslator.ProtoToTraces([]*jaegermodel.Batch{{
		Process: jaegermodel.NewProcess("", jaegerKeyValues(
			"jaeger.version", "unknown",
			"service.version", "process_tag_value",
		)),
		Spans: []*jaegermodel.Span{{
			TraceID: jaegermodel.NewTraceID(0, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(456),
		}, {
			TraceID: jaegermodel.NewTraceID(0, 0x000046467830),
			SpanID:  jaegermodel.NewSpanID(456),
			Tags: []jaegermodel.KeyValue{
				jaegerKeyValue("service.version", "span_tag_value"),
			},
		}},
	}})
	require.NoError(t, err)

	var batches []*modelpb.Batch
	recorder := batchRecorderBatchProcessor(&batches)
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: recorder,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
	require.NoError(t, err)
	require.Equal(t, otlp.ConsumeTracesResult{}, result)

	batch := *batches[0]
	assert.Equal(t, "process_tag_value", batch[0].Service.Version)
	assert.Equal(t, "span_tag_value", batch[1].Service.Version)
}

/* TODO
func TestTracesLogging(t *testing.T) {
	for _, level := range []logp.Level{logp.InfoLevel, logp.DebugLevel} {
		t.Run(level.String(), func(t *testing.T) {
			logp.DevelopmentSetup(logp.ToObserverOutput(), logp.WithLevel(level))
			transformTraces(t, ptrace.NewTraces())
			logs := logp.ObserverLogs().TakeAll()
			if level == logp.InfoLevel {
				assert.Empty(t, logs)
			} else {
				assert.NotEmpty(t, logs)
			}
		})
	}
}
*/

func TestServiceTarget(t *testing.T) {
	test := func(t *testing.T, expected *modelpb.ServiceTarget, input map[string]interface{}) {
		t.Helper()
		event := transformSpanWithAttributes(t, input)
		assert.Empty(t, cmp.Diff(expected, event.Service.Target, protocmp.Transform()))
	}

	setAttr := func(t *testing.T, baseAttr map[string]any, key string, val any) map[string]any {
		t.Helper()
		newAttr := make(map[string]any)
		// Copy from the original map to the target map
		for key, value := range baseAttr {
			newAttr[key] = value
		}
		newAttr[key] = val
		return newAttr
	}
	t.Run("db_spans_with_peerservice_system", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Type: "postgresql",
			Name: "testsvc",
		}, map[string]interface{}{
			"peer.service": "testsvc",
			"db.system":    "postgresql",
		})
	})

	t.Run("db_spans_with_peerservice_name_system", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Type: "postgresql",
			Name: "testdb",
		}, map[string]interface{}{
			"peer.service": "testsvc",
			"db.name":      "testdb",
			"db.system":    "postgresql",
		})
	})

	t.Run("db_spans_with_name", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Type: "db",
			Name: "testdb",
		}, map[string]interface{}{
			"db.name": "testdb",
		})
	})

	t.Run("http_spans_with_peerservice_url", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "test-url:443",
			Type: "http",
		}, map[string]interface{}{
			"peer.service": "testsvc",
			"http.url":     "https://test-url:443/",
		})
	})

	t.Run("http_spans_with_scheme_host_target", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "test-url:443",
			Type: "http",
		}, map[string]interface{}{
			"http.scheme": "https",
			"http.host":   "test-url:443",
			"http.target": "/",
		})
	})

	t.Run("http_spans_with_scheme_netpeername_netpeerport_target", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "test-url:443",
			Type: "http",
		}, map[string]interface{}{
			"http.scheme":   "https",
			"net.peer.name": "test-url",
			"net.peer.ip":   "::1", // net.peer.name preferred
			"net.peer.port": 443,
			"http.target":   "/",
		})
	})

	t.Run("http_spans_with_scheme_netpeerip_netpeerport_target", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "[::1]:443",
			Type: "http",
		}, map[string]interface{}{
			"http.scheme":   "https",
			"net.peer.ip":   "::1", // net.peer.name preferred
			"net.peer.port": 443,
			"http.target":   "/",
		})
	})

	t.Run("rpc_spans_with_peerservice_system", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "testsvc",
			Type: "grpc",
		}, map[string]interface{}{
			"peer.service": "testsvc",
			"rpc.system":   "grpc",
		})
	})

	t.Run("rpc_spans_with_peerservice_system_service", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "test",
			Type: "grpc",
		}, map[string]interface{}{
			"peer.service": "testsvc",
			"rpc.system":   "grpc",
			"rpc.service":  "test",
		})
	})

	t.Run("rpc_spans_with_service", func(t *testing.T) {
		test(t, &modelpb.ServiceTarget{
			Name: "test",
			Type: "external",
		}, map[string]interface{}{
			"rpc.service": "test",
		})
	})

	t.Run("messaging_spans_with_peerservice_system_destination", func(t *testing.T) {
		baseAttr := map[string]any{
			"peer.service":     "testsvc",
			"messaging.system": "kafka",
		}
		for _, attr := range []map[string]any{
			setAttr(t, baseAttr, "messaging.destination", "myTopic"),
			setAttr(t, baseAttr, "messaging.destination.name", "myTopic"),
		} {
			test(t, &modelpb.ServiceTarget{
				Name: "myTopic",
				Type: "kafka",
			}, attr)
		}
	})

	t.Run("messaging_spans_with_peerservice_system_destination_tempdestination", func(t *testing.T) {
		baseAttr := map[string]any{
			"peer.service":               "testsvc",
			"messaging.temp_destination": true,
			"messaging.system":           "kafka",
		}
		for _, attr := range []map[string]any{
			setAttr(t, baseAttr, "messaging.destination", "myTopic"),
			setAttr(t, baseAttr, "messaging.destination.name", "myTopic"),
		} {
			test(t, &modelpb.ServiceTarget{
				Name: "testsvc",
				Type: "kafka",
			}, attr)
		}
	})

	t.Run("messaging_spans_with_destination", func(t *testing.T) {
		for _, attr := range []map[string]any{
			{"messaging.destination": "myTopic"},
			{"messaging.destination.name": "myTopic"},
		} {
			test(t, &modelpb.ServiceTarget{
				Name: "myTopic",
				Type: "messaging",
			}, attr)
		}
	})
}

func TestGRPCTransactionFromNodejsSDK(t *testing.T) {
	t.Run("transaction transformation", func(t *testing.T) {
		test := func(t *testing.T, input map[string]interface{}) {
			t.Helper()
			event := transformTransactionWithAttributes(t, input, func(s ptrace.Span) {
				s.SetKind(ptrace.SpanKindServer)
			})
			assert.Equal(t, "request", event.Transaction.Type)
		}
		test(t, map[string]interface{}{
			"rpc.grpc.status_code": int64(codes.Unavailable),
		})
	})

	t.Run("span transformation", func(t *testing.T) {
		event := transformSpanWithAttributes(t, map[string]interface{}{
			"rpc.grpc.status_code": int64(codes.Unavailable),
		})
		assert.Equal(t, "external", event.Span.Type)
		assert.Equal(t, "grpc", event.Span.Subtype)
	})
}

func TestSpanCodeStacktrace(t *testing.T) {
	t.Run("code stacktrace", func(t *testing.T) {
		event := transformSpanWithAttributes(t, map[string]interface{}{
			"code.stacktrace": "stacktrace value",
		})
		assert.Equal(t, "stacktrace value", event.Code.Stacktrace)
	})
}

func TestSpanEventsDataStream(t *testing.T) {
	for _, isException := range []bool{false, true} {
		t.Run(fmt.Sprintf("isException=%v", isException), func(t *testing.T) {
			timestamp := time.Unix(123, 0).UTC()

			traces, spans := newTracesSpans()
			traces.ResourceSpans().At(0).Resource().Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
			traces.ResourceSpans().At(0).Resource().Attributes().PutStr("data_stream.dataset", "1")
			traces.ResourceSpans().At(0).Resource().Attributes().PutStr("data_stream.namespace", "2")
			otelSpan := spans.Spans().AppendEmpty()
			otelSpan.SetTraceID(pcommon.TraceID{1})
			otelSpan.SetSpanID(pcommon.SpanID{2})
			otelSpan.Attributes().PutStr("data_stream.dataset", "3")
			otelSpan.Attributes().PutStr("data_stream.namespace", "4")

			spanEvent := ptrace.NewSpanEvent()
			spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			if isException {
				spanEvent.SetName("exception")
				spanEvent.Attributes().PutStr("exception.type", "java.net.ConnectException.OSError")
				spanEvent.Attributes().PutStr("exception.message", "Division by zero")
			}

			spanEvent.Attributes().PutStr("data_stream.dataset", "5")
			spanEvent.Attributes().PutStr("data_stream.namespace", "6")
			spanEvent.CopyTo(otelSpan.Events().AppendEmpty())

			allEvents := transformTraces(t, traces)
			events := (*allEvents)[1:]
			assert.Equal(t, &modelpb.DataStream{
				Dataset:   "5",
				Namespace: "6",
			}, events[0].DataStream)
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
			"message", "retrying connection",
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

func batchRecorderBatchProcessor(out *[]*modelpb.Batch) modelpb.BatchProcessor {
	return modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		batchCopy := batch.Clone()
		*out = append(*out, &batchCopy)
		return nil
	})
}

func encodeBatch(t testing.TB, batches ...*modelpb.Batch) [][]byte {
	var docs [][]byte
	for _, batch := range batches {
		for _, event := range *batch {
			var w fastjson.Writer
			err := modeljson.MarshalAPMEvent(event, &w)
			require.NoError(t, err)
			data := w.Bytes()
			docs = append(docs, data)
		}
	}
	return docs
}

// TODO(axw) don't use approval testing here. The input should only care about the
// conversion to modelpb.APMEvent, not the final Elasticsearch document encoding.
func approveEventDocs(t testing.TB, name string, docs [][]byte) {
	t.Helper()

	events := make([]any, len(docs))
	for i, doc := range docs {
		var m map[string]any
		if err := json.Unmarshal(doc, &m); err != nil {
			t.Fatal(err)
		}

		// Ignore the specific value for "event.received", as it is dynamic.
		// All received events should have this.
		require.Contains(t, m, "event")
		event := m["event"].(map[string]any)
		require.Contains(t, event, "received")
		delete(event, "received")
		if len(event) == 0 {
			delete(m, "event")
		}

		if e, ok := m["error"].(map[string]any); ok {
			if _, ok := e["id"]; ok {
				e["id"] = "dynamic"
			}
		}

		events[i] = m
	}
	received := map[string]any{"events": events}

	var approved any
	approvedData, err := os.ReadFile(filepath.Join("test_approved", name+".approved.json"))
	require.NoError(t, err)
	if err := json.Unmarshal(approvedData, &approved); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(approved, received); diff != "" {
		t.Fatalf("%s\n", diff)
	}
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

func transformTransactionWithAttributes(t *testing.T, attrs map[string]interface{}, configFns ...func(ptrace.Span)) *modelpb.APMEvent {
	traces, spans := newTracesSpans()
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	for _, fn := range configFns {
		fn(otelSpan)
	}
	otelSpan.Attributes().FromRaw(attrs)
	events := transformTraces(t, traces)
	return (*events)[0]
}

func transformSpanWithAttributes(t *testing.T, attrs map[string]interface{}, configFns ...func(ptrace.Span)) *modelpb.APMEvent {
	traces, spans := newTracesSpans()
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	otelSpan.SetParentSpanID(pcommon.SpanID{3})
	for _, fn := range configFns {
		fn(otelSpan)
	}
	otelSpan.Attributes().FromRaw(attrs)
	events := transformTraces(t, traces)
	return (*events)[0]
}

func transformTransactionSpanEvents(t *testing.T, language string, spanEvents ...ptrace.SpanEvent) (transaction *modelpb.APMEvent, events []*modelpb.APMEvent) {
	traces, spans := newTracesSpans()
	traces.ResourceSpans().At(0).Resource().Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, language)
	otelSpan := spans.Spans().AppendEmpty()
	otelSpan.SetTraceID(pcommon.TraceID{1})
	otelSpan.SetSpanID(pcommon.SpanID{2})
	for _, spanEvent := range spanEvents {
		spanEvent.CopyTo(otelSpan.Events().AppendEmpty())
	}

	allEvents := transformTraces(t, traces)
	require.NotEmpty(t, allEvents)
	return (*allEvents)[0], (*allEvents)[1:]
}

func transformTraces(t *testing.T, traces ptrace.Traces) *modelpb.Batch {
	var processed modelpb.Batch
	processor := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		return nil
	})
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	_, err := consumer.ConsumeTracesWithResult(context.Background(), traces)
	require.NoError(t, err)
	return &processed
}

func newTracesSpans() (ptrace.Traces, ptrace.ScopeSpans) {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	return traces, scopeSpans
}

func newUint32(v uint32) *uint32 {
	return &v
}

func newBool(v bool) *bool {
	return &v
}
