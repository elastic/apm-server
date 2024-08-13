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

package v2

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/nullable"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestDecodeNestedSpan(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		defaultVal := modeldecodertest.DefaultValues()
		_, eventBase := initializedInputMetadata(defaultVal)
		input := modeldecoder.Input{Base: eventBase}
		str := `{"span":{"duration":100,"id":"a-b-c","name":"s","parent_id":"parent-123","trace_id":"trace-ab","type":"db","start":143}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch modelpb.Batch
		require.NoError(t, DecodeNestedSpan(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Span)
		assert.Equal(t, eventBase.Timestamp+uint64((143*time.Millisecond).Nanoseconds()), batch[0].Timestamp)
		assert.Equal(t, uint64(100*time.Millisecond), batch[0].Event.Duration)
		assert.Equal(t, "parent-123", batch[0].ParentId, protocmp.Transform())
		assert.Equal(t, &modelpb.Trace{Id: "trace-ab"}, batch[0].Trace, protocmp.Transform())
		assert.Empty(t, cmp.Diff(&modelpb.Span{
			Name:                "s",
			Type:                "db",
			Id:                  "a-b-c",
			RepresentativeCount: 1,
		}, batch[0].Span, protocmp.Transform()))

		err := DecodeNestedSpan(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
		err := DecodeNestedSpan(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToSpanModel(t *testing.T) {
	t.Run("set-metadata", func(t *testing.T) {
		exceptions := func(key string) bool { return false }
		var input span
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		_, out := initializedInputMetadata(defaultVal)
		mapToSpanModel(&input, out)
		modeldecodertest.AssertStructValues(t, &out.Service, exceptions, defaultVal)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			if strings.HasPrefix(key, "links") {
				// Links are tested below in the 'links' test.
				return true
			}
			switch key {
			case
				"message.headers.key",
				"message.headers.value",
				"composite.compression_strategy",
				// RepresentativeCount is tested further down in test 'sample-rate'
				"representative_count",
				// Kind is tested further down
				"kind",

				// Derived using service.target.*
				"destination_service.Resource",

				// Not set for spans:
				"destination_service.response_time",
				"destination_service.ResponseTime.Count",
				"destination_service.ResponseTime.Sum",
				"self_time",
				"SelfTime.Count",
				"SelfTime.Sum":
				return true
			}
			for _, s := range []string{
				// stacktrace values are set when sourcemapping is applied
				"stacktrace.original",
				"stacktrace.sourcemap",
				"stacktrace.exclude_from_grouping"} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input span
		var out1, out2 modelpb.APMEvent
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.OTel.Reset()
		mapToSpanModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)

		// reuse input model for different event
		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		input.OTel.Reset()
		mapToSpanModel(&input, &out2)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out2.Span, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)
	})

	t.Run("outcome", func(t *testing.T) {
		var input span
		var out modelpb.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.HTTP.StatusCode.Set(http.StatusPermanentRedirect)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from other fields - success
		input.Outcome.Reset()
		input.Context.HTTP.StatusCode.Set(http.StatusPermanentRedirect)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
		// derive from other fields - failure
		input.Outcome.Reset()
		input.Context.HTTP.StatusCode.Set(http.StatusBadRequest)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from other fields - unknown
		input.Outcome.Reset()
		input.OTel.Reset()
		input.Context.HTTP.StatusCode.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
		// outcome is success when not assigned and it's otel
		input.Outcome.Reset()
		input.OTel.SpanKind.Set(spanKindInternal)
		input.Context.HTTP.StatusCode.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
	})

	t.Run("timestamp", func(t *testing.T) {
		// add start to base event timestamp if event timestamp is unspecified and start is given
		var input span
		var out modelpb.APMEvent
		reqTime := time.Now().Add(time.Hour).UTC()
		out.Timestamp = modelpb.FromTime(reqTime)
		input.Start.Set(20.5)
		mapToSpanModel(&input, &out)
		timestamp := reqTime.Add(time.Duration(input.Start.Val * float64(time.Millisecond))).UTC()
		assert.Equal(t, modelpb.FromTime(timestamp), out.Timestamp)

		// leave base event timestamp unmodified if neither event timestamp nor start is specified
		out = modelpb.APMEvent{Timestamp: modelpb.FromTime(reqTime)}
		input.Start.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, modelpb.FromTime(reqTime), out.Timestamp)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input span
		var out modelpb.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.OTel.Reset()
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToSpanModel(&input, &out)
		assert.Equal(t, 4.0, out.Span.RepresentativeCount)
		// sample rate is not set, default representative count should be 1
		out.Span.RepresentativeCount = 0.0
		input.SampleRate.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, 1.0, out.Span.RepresentativeCount)
		// sample rate is set to 0
		input.SampleRate.Set(0)
		mapToSpanModel(&input, &out)
		assert.Equal(t, 0.0, out.Span.RepresentativeCount)
	})

	t.Run("type-subtype-action", func(t *testing.T) {
		for _, tc := range []struct {
			name                                 string
			inputType, inputSubtype, inputAction string
			typ, subtype, action                 string
		}{
			{name: "only-type", inputType: "xyz",
				typ: "xyz"},
			{name: "derive-subtype", inputType: "x.y",
				typ: "x", subtype: "y"},
			{name: "derive-subtype-action", inputType: "x.y.z.a",
				typ: "x", subtype: "y", action: "z.a"},
			{name: "type-subtype", inputType: "x.y.z", inputSubtype: "a",
				typ: "x.y.z", subtype: "a"},
			{name: "type-action", inputType: "x.y.z", inputAction: "b",
				typ: "x.y.z", action: "b"},
			{name: "type-subtype-action", inputType: "x.y", inputSubtype: "a", inputAction: "b",
				typ: "x.y", subtype: "a", action: "b"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var input span
				defaultVal := modeldecodertest.DefaultValues()
				modeldecodertest.SetStructValues(&input, defaultVal)
				input.Type.Set(tc.inputType)
				if tc.inputSubtype != "" {
					input.Subtype.Set(tc.inputSubtype)
				} else {
					input.Subtype.Reset()
				}
				if tc.inputAction != "" {
					input.Action.Set(tc.inputAction)
				} else {
					input.Action.Reset()
				}
				var out modelpb.APMEvent
				mapToSpanModel(&input, &out)
				assert.Equal(t, tc.typ, out.Span.Type)
				assert.Equal(t, tc.subtype, out.Span.Subtype)
				assert.Equal(t, tc.action, out.Span.Action)
			})
		}
	})

	t.Run("http-request", func(t *testing.T) {
		var input span
		input.Context.HTTP.Request.ID.Set("some-request-id")
		var out modelpb.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, "some-request-id", out.Http.Request.Id)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input span
		input.Context.HTTP.Response.Headers.Set(http.Header{"a": []string{"b", "c"}})
		var out modelpb.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, []*modelpb.HTTPHeader{
			{
				Key:   "a",
				Value: []string{"b", "c"},
			},
		}, out.Http.Response.Headers)
	})

	t.Run("otel-bridge", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			expected := modelpb.URL{
				Original: "https://testing.invalid:80/foo?bar",
			}
			attrs := map[string]interface{}{
				"http.scheme":   "https",
				"net.peer.name": "testing.invalid",
				"net.peer.ip":   "::1", // net.peer.name preferred
				"net.peer.port": json.Number("80"),
				"http.target":   "/foo?bar",
			}
			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToSpanModel(&input, &event)
			assert.Equal(t, &expected, event.Url)
			assert.Equal(t, "CLIENT", event.Span.Kind)
			assert.Empty(t, cmp.Diff(&modelpb.ServiceTarget{
				Type: "http",
				Name: "testing.invalid:80",
			}, event.Service.Target, protocmp.Transform()))
		})

		t.Run("http-destination", func(t *testing.T) {
			expectedDestination := modelpb.Destination{
				Address: "testing.invalid",
				Port:    443,
			}
			expectedDestinationService := &modelpb.DestinationService{
				Type:     "external",
				Name:     "https://testing.invalid",
				Resource: "testing.invalid:443",
			}
			attrs := map[string]interface{}{
				"http.url": "https://testing.invalid:443/foo?bar",
			}

			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToSpanModel(&input, &event)
			assert.Equal(t, &expectedDestination, event.Destination)
			assert.Empty(t, cmp.Diff(expectedDestinationService, event.Span.DestinationService, protocmp.Transform()))
			assert.Equal(t, "CLIENT", event.Span.Kind)
			assert.Empty(t, cmp.Diff(&modelpb.ServiceTarget{
				Type: "http",
				Name: "testing.invalid:443",
			}, event.Service.Target, protocmp.Transform()))
		})

		t.Run("db", func(t *testing.T) {
			connectionString := "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;"
			attrs := map[string]interface{}{
				"db.system":            "mysql",
				"db.connection_string": connectionString,
				"db.user":              "billing_user",
				"db.name":              "ShopDb",
				"db.statement":         "SELECT * FROM orders WHERE order_id = 'o4711'",
				"net.peer.name":        "shopdb.example.com",
				"net.peer.ip":          "192.0.2.12",
				"net.peer.port":        3306,
				"net.transport":        "IP.TCP",
			}

			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()
			input.Action.Reset()
			mapToSpanModel(&input, &event)

			assert.Equal(t, "db", event.Span.Type)
			assert.Equal(t, "mysql", event.Span.Subtype)
			assert.Equal(t, "", event.Span.Action)
			assert.Equal(t, "CLIENT", event.Span.Kind)
			assert.Equal(t, &modelpb.DB{
				Instance:  "ShopDb",
				Statement: "SELECT * FROM orders WHERE order_id = 'o4711'",
				Type:      "mysql",
				UserName:  "billing_user",
			}, event.Span.Db)
			assert.Empty(t, cmp.Diff(&modelpb.ServiceTarget{
				Type: "mysql",
				Name: "ShopDb",
			}, event.Service.Target, protocmp.Transform()))
		})

		t.Run("rpc", func(t *testing.T) {
			attrs := map[string]interface{}{
				"rpc.system":           "grpc",
				"rpc.service":          "myservice.EchoService",
				"rpc.method":           "exampleMethod",
				"rpc.grpc.status_code": json.Number(strconv.Itoa(int(codes.Unavailable))),
				"net.peer.ip":          "10.20.30.40",
				"net.peer.port":        json.Number("123"),
			}

			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()
			input.Action.Reset()
			mapToSpanModel(&input, &event)

			assert.Equal(t, "external", event.Span.Type)
			assert.Equal(t, "grpc", event.Span.Subtype)
			assert.Equal(t, &modelpb.Destination{
				Address: "10.20.30.40",
				Port:    123,
			}, event.Destination)
			assert.Equal(t, "CLIENT", event.Span.Kind)
			assert.Empty(t, cmp.Diff(&modelpb.DestinationService{
				Type:     "external",
				Name:     "10.20.30.40:123",
				Resource: "10.20.30.40:123",
			}, event.Span.DestinationService, protocmp.Transform()))
			assert.Empty(t, cmp.Diff(&modelpb.ServiceTarget{
				Type: "grpc",
				Name: "myservice.EchoService",
			}, event.Service.Target, protocmp.Transform()))
		})

		t.Run("messaging", func(t *testing.T) {
			for _, attrs := range []map[string]any{
				{
					"messaging.system":      "kafka",
					"messaging.destination": "myTopic",
					"net.peer.ip":           "10.20.30.40",
					"net.peer.port":         json.Number("123"),
				},
				{
					"messaging.system":           "kafka",
					"messaging.destination.name": "myTopic",
					"net.peer.ip":                "10.20.30.40",
					"net.peer.port":              json.Number("123"),
				},
			} {

				var input span
				var event modelpb.APMEvent
				modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
				input.OTel.Attributes = attrs
				input.OTel.SpanKind.Set("PRODUCER")
				input.Type.Reset()
				mapToSpanModel(&input, &event)

				assert.Equal(t, "messaging", event.Span.Type)
				assert.Equal(t, "kafka", event.Span.Subtype)
				assert.Equal(t, "send", event.Span.Action)
				assert.Equal(t, "PRODUCER", event.Span.Kind)
				assert.Equal(t, &modelpb.Destination{
					Address: "10.20.30.40",
					Port:    123,
				}, event.Destination)
				assert.Empty(t, cmp.Diff(&modelpb.DestinationService{
					Type:     "messaging",
					Name:     "kafka",
					Resource: "kafka/myTopic",
				}, event.Span.DestinationService, protocmp.Transform()))
				assert.Empty(t, cmp.Diff(&modelpb.ServiceTarget{
					Type: "kafka",
					Name: "myTopic",
				}, event.Service.Target, protocmp.Transform()))
			}
		})

		t.Run("network", func(t *testing.T) {
			attrs := map[string]interface{}{
				"network.connection.type":    "cell",
				"network.connection.subtype": "LTE",
				"network.carrier.name":       "Vodafone",
				"network.carrier.mnc":        "01",
				"network.carrier.mcc":        "101",
				"network.carrier.icc":        "UK",
			}
			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.Type.Reset()
			mapToSpanModel(&input, &event)

			expected := &modelpb.Network{
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
			assert.Equal(t, expected, event.Network)
			assert.Equal(t, "INTERNAL", event.Span.Kind)
		})

		t.Run("kind", func(t *testing.T) {
			var input span
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.SpanKind.Set("CONSUMER")

			mapToSpanModel(&input, &event)
			assert.Equal(t, "CONSUMER", event.Span.Kind)
		})
	})

	t.Run("composite", func(t *testing.T) {
		var input span
		var event modelpb.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())

		input.Composite.CompressionStrategy.Set("invalid")
		mapToSpanModel(&input, &event)
		assert.Equal(t,
			modelpb.CompressionStrategy_COMPRESSION_STRATEGY_UNSPECIFIED,
			event.Span.Composite.CompressionStrategy,
		)

		input.Composite.CompressionStrategy.Set("exact_match")
		mapToSpanModel(&input, &event)
		assert.Equal(t,
			modelpb.CompressionStrategy_COMPRESSION_STRATEGY_EXACT_MATCH,
			event.Span.Composite.CompressionStrategy,
		)
	})

	t.Run("labels", func(t *testing.T) {
		var input span
		input.Context.Tags = map[string]any{
			"a": "b",
			"c": float64(12315124131),
			"d": 12315124131.12315124131,
			"e": true,
		}
		var out modelpb.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, modelpb.Labels{
			"a": {Value: "b"},
			"e": {Value: "true"},
		}, modelpb.Labels(out.Labels))
		assert.Equal(t, modelpb.NumericLabels{
			"c": {Value: float64(12315124131)},
			"d": {Value: float64(12315124131.12315124131)},
		}, modelpb.NumericLabels(out.NumericLabels))
	})

	t.Run("links", func(t *testing.T) {
		var input span
		input.Links = []spanLink{{
			SpanID:  nullable.String{Val: "span1"},
			TraceID: nullable.String{Val: "trace1"},
		}, {
			SpanID:  nullable.String{Val: "span2"},
			TraceID: nullable.String{Val: "trace2"},
		}}
		var out modelpb.APMEvent
		mapToSpanModel(&input, &out)

		assert.Equal(t, []*modelpb.SpanLink{{
			SpanId:  "span1",
			TraceId: "trace1",
		}, {
			SpanId:  "span2",
			TraceId: "trace2",
		}}, out.Span.Links)
	})

	t.Run("service-target", func(t *testing.T) {
		for _, tc := range []struct {
			name                         string
			inTargetType, inTargetName   string
			outTargetType, outTargetName string
			resource                     string
		}{
			{name: "passed-as-input", inTargetType: "some-type", outTargetType: "some-type", outTargetName: ""},
			{name: "infer-from-resource", resource: "postgres/testdb", outTargetType: "postgres", outTargetName: "testdb"},
			{name: "infer-only-type-from-resource", resource: "mysql", outTargetType: "mysql", outTargetName: ""},
			{name: "infer-only-name-from-resource", resource: "my-db", outTargetType: "", outTargetName: "my-db"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var input span
				defaultVal := modeldecodertest.DefaultValues()
				modeldecodertest.SetStructValues(&input, defaultVal)
				if tc.inTargetType != "" {
					input.Context.Service.Target.Type.Set(tc.inTargetType)
				} else {
					input.Context.Service.Target.Type.Reset()
				}
				if tc.inTargetName != "" {
					input.Context.Service.Target.Name.Set(tc.inTargetName)
				} else {
					input.Context.Service.Target.Name.Reset()
				}
				if tc.resource != "" {
					input.Context.Destination.Service.Resource.Set(tc.resource)
				} else {
					input.Context.Destination.Service.Resource.Reset()
				}
				var out modelpb.APMEvent
				mapToSpanModel(&input, &out)
				assert.Equal(t, tc.outTargetType, out.Service.Target.Type)
				assert.Equal(t, tc.outTargetName, out.Service.Target.Name)
			})
		}
	})
}
