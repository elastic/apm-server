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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestResetSpanOnRelease(t *testing.T) {
	inp := `{"span":{"name":"tr-a"}}`
	root := fetchSpanRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseSpanRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedSpan(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		defaultVal := modeldecodertest.DefaultValues()
		_, eventBase := initializedInputMetadata(defaultVal)
		input := modeldecoder.Input{Base: eventBase}
		str := `{"span":{"duration":100,"id":"a-b-c","name":"s","parent_id":"parent-123","trace_id":"trace-ab","type":"db","start":143}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedSpan(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Span)
		defaultVal.Update(time.Time{}.Add(143 * time.Millisecond))
		modeldecodertest.AssertStructValues(t, &batch[0], isMetadataException, defaultVal)

		err := DecodeNestedSpan(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
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
		mapToSpanModel(&input, &out)
		modeldecodertest.AssertStructValues(t, &out.Service, exceptions, defaultVal)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			switch key {
			case
				// RepresentativeCount is tested further down in test 'sample-rate'
				"RepresentativeCount",
				// Kind is tested further down
				"Kind",

				// Not set for spans:
				"DestinationService.ResponseTime",
				"DestinationService.ResponseTime.Count",
				"DestinationService.ResponseTime.Sum",
				"SelfTime",
				"SelfTime.Count",
				"SelfTime.Sum":
				return true
			}
			for _, s := range []string{
				// stacktrace values are set when sourcemapping is applied
				"Stacktrace.Original",
				"Stacktrace.Sourcemap",
				"Stacktrace.ExcludeFromGrouping"} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input span
		var out1, out2 model.APMEvent
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
		var out model.APMEvent
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
		input.Context.HTTP.StatusCode.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
	})

	t.Run("timestamp", func(t *testing.T) {
		var input span
		var out model.APMEvent
		reqTime := time.Now().Add(time.Hour)
		// add start to requestTime if eventTime is zero and start is given
		defaultVal := modeldecodertest.DefaultValues()
		defaultVal.Update(20.5, time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		out.Timestamp = reqTime
		mapToSpanModel(&input, &out)
		timestamp := reqTime.Add(time.Duration((20.5) * float64(time.Millisecond)))
		assert.Equal(t, timestamp, out.Timestamp)
		// leave event timestamp unmodified if eventTime is zero and start is not set
		out = model.APMEvent{Timestamp: reqTime}
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Start.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, reqTime, out.Timestamp)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input span
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.OTel.Reset()
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToSpanModel(&input, &out)
		assert.Equal(t, 4.0, out.Span.RepresentativeCount)
		// sample rate is not set
		out.Span.RepresentativeCount = 0.0
		input.SampleRate.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, 0.0, out.Span.RepresentativeCount)
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
				var out model.APMEvent
				mapToSpanModel(&input, &out)
				assert.Equal(t, tc.typ, out.Span.Type)
				assert.Equal(t, tc.subtype, out.Span.Subtype)
				assert.Equal(t, tc.action, out.Span.Action)
			})
		}
	})

	t.Run("http-headers", func(t *testing.T) {
		var input span
		input.Context.HTTP.Response.Headers.Set(http.Header{"a": []string{"b", "c"}})
		var out model.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b", "c"}}, out.HTTP.Response.Headers)
	})

	t.Run("otel-bridge", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			expected := model.URL{
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
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToSpanModel(&input, &event)
			assert.Equal(t, expected, event.URL)
			assert.Equal(t, "CLIENT", event.Span.Kind)
		})

		t.Run("http-destination", func(t *testing.T) {
			expectedDestination := model.Destination{
				Address: "testing.invalid",
				Port:    443,
			}
			expectedDestinationService := &model.DestinationService{
				Type:     "external",
				Name:     "https://testing.invalid",
				Resource: "testing.invalid:443",
			}
			attrs := map[string]interface{}{
				"http.url": "https://testing.invalid:443/foo?bar",
			}

			var input span
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToSpanModel(&input, &event)
			assert.Equal(t, expectedDestination, event.Destination)
			assert.Equal(t, expectedDestinationService, event.Span.DestinationService)
			assert.Equal(t, "CLIENT", event.Span.Kind)
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
			var event model.APMEvent
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

			assert.Equal(t, &model.DB{
				Instance:  "ShopDb",
				Statement: "SELECT * FROM orders WHERE order_id = 'o4711'",
				Type:      "mysql",
				UserName:  "billing_user",
			}, event.Span.DB)
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
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()
			input.Action.Reset()
			mapToSpanModel(&input, &event)

			assert.Equal(t, "external", event.Span.Type)
			assert.Equal(t, "grpc", event.Span.Subtype)
			assert.Equal(t, model.Destination{
				Address: "10.20.30.40",
				Port:    123,
			}, event.Destination)
			assert.Equal(t, "CLIENT", event.Span.Kind)
			assert.Equal(t, &model.DestinationService{
				Type:     "external",
				Name:     "10.20.30.40:123",
				Resource: "10.20.30.40:123",
			}, event.Span.DestinationService)
		})

		t.Run("messaging", func(t *testing.T) {
			attrs := map[string]interface{}{
				"messaging.system":      "kafka",
				"messaging.destination": "myTopic",
				"net.peer.ip":           "10.20.30.40",
				"net.peer.port":         json.Number("123"),
			}

			var input span
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Set("PRODUCER")
			input.Type.Reset()
			mapToSpanModel(&input, &event)

			assert.Equal(t, "messaging", event.Span.Type)
			assert.Equal(t, "kafka", event.Span.Subtype)
			assert.Equal(t, "send", event.Span.Action)
			assert.Equal(t, "PRODUCER", event.Span.Kind)
			assert.Equal(t, model.Destination{
				Address: "10.20.30.40",
				Port:    123,
			}, event.Destination)
			assert.Equal(t, &model.DestinationService{
				Type:     "messaging",
				Name:     "kafka",
				Resource: "kafka/myTopic",
			}, event.Span.DestinationService)

		})

		t.Run("network", func(t *testing.T) {
			attrs := map[string]interface{}{
				"net.host.connection.type":    "cell",
				"net.host.connection.subtype": "LTE",
				"net.host.carrier.name":       "Vodafone",
				"net.host.carrier.mnc":        "01",
				"net.host.carrier.mcc":        "101",
				"net.host.carrier.icc":        "UK",
			}
			var input span
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.Type.Reset()
			mapToSpanModel(&input, &event)

			expected := model.Network{
				Connection: model.NetworkConnection{
					Type:    "cell",
					Subtype: "LTE",
				},
				Carrier: model.NetworkCarrier{
					Name: "Vodafone",
					MNC:  "01",
					MCC:  "101",
					ICC:  "UK",
				},
			}
			assert.Equal(t, expected, event.Network)
			assert.Equal(t, "INTERNAL", event.Span.Kind)
		})

		t.Run("kind", func(t *testing.T) {
			var input span
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.SpanKind.Set("CONSUMER")

			mapToSpanModel(&input, &event)
			assert.Equal(t, "CONSUMER", event.Span.Kind)
		})
	})
	t.Run("labels", func(t *testing.T) {
		var input span
		input.Context.Tags = common.MapStr{
			"a": "b",
			"c": float64(12315124131),
			"d": 12315124131.12315124131,
			"e": true,
		}
		var out model.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, common.MapStr{
			"a": "b",
			"e": "true",
		}, out.Labels)
		assert.Equal(t, common.MapStr{
			"c": float64(12315124131),
			"d": float64(12315124131.12315124131),
		}, out.NumericLabels)
	})
}
