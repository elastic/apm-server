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
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestResetTransactionOnRelease(t *testing.T) {
	inp := `{"transaction":{"name":"tr-a"}}`
	root := fetchTransactionRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseTransactionRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{}
		str := `{"transaction":{"duration":100,"timestamp":1599996822281000,"id":"100","trace_id":"1","type":"request","span_count":{"started":2}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))

		var batch model.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Transaction)
		assert.Equal(t, "request", batch[0].Transaction.Type)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", batch[0].Timestamp.String())

		input = modeldecoder.Input{Base: model.APMEvent{Timestamp: now}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = model.Batch{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		// if no timestamp is provided, fall back to base event timestamp
		assert.Equal(t, now, batch[0].Timestamp)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input transaction
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, &out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Client.IP, out.Client.IP.String())
		assert.Equal(t, common.MapStr{
			"init0": "init", "init1": "init", "init2": "init",
			"overwritten0": "overwritten", "overwritten1": "overwritten",
		}, out.Labels)
		//assert.Equal(t, tLabels, out.Transaction.Labels)
		exceptions := func(key string) bool { return false }
		modeldecodertest.AssertStructValues(t, &out.Service, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out.User, exceptions, otherVal)
	})

	t.Run("cloud.origin", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		origin := contextCloudOrigin{}
		origin.Account.ID.Set("accountID")
		origin.Provider.Set("aws")
		origin.Region.Set("us-east-1")
		origin.Service.Name.Set("serviceName")
		input.Context.Cloud.Origin = origin
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "accountID", out.Cloud.Origin.AccountID)
		assert.Equal(t, "aws", out.Cloud.Origin.Provider)
		assert.Equal(t, "us-east-1", out.Cloud.Origin.Region)
		assert.Equal(t, "serviceName", out.Cloud.Origin.ServiceName)
	})

	t.Run("service.origin", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		origin := contextServiceOrigin{}
		origin.ID.Set("abc123")
		origin.Name.Set("name")
		origin.Version.Set("1.0")
		input.Context.Service.Origin = origin
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "abc123", out.Service.Origin.ID)
		assert.Equal(t, "name", out.Service.Origin.Name)
		assert.Equal(t, "1.0", out.Service.Origin.Version)
	})

	t.Run("faas", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		input.FAAS.Coldstart.Set(true)
		input.FAAS.Execution.Set("execution")
		input.FAAS.Trigger.Type.Set("http")
		input.FAAS.Trigger.RequestID.Set("abc123")
		mapToTransactionModel(&input, &out)
		assert.True(t, *out.FAAS.Coldstart)
		assert.Equal(t, "execution", out.FAAS.Execution)
		assert.Equal(t, "http", out.FAAS.TriggerType)
		assert.Equal(t, "abc123", out.FAAS.TriggerRequestID)
	})

	t.Run("dropped_span_stats", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		var esDss, mysqlDss transactionDroppedSpanStats

		durationSumUs := 10_290_000
		esDss.DestinationServiceResource.Set("https://elasticsearch:9200")
		esDss.Outcome.Set("success")
		esDss.Duration.Count.Set(2)
		esDss.Duration.Sum.Us.Set(durationSumUs)
		mysqlDss.DestinationServiceResource.Set("mysql://mysql:3306")
		mysqlDss.Outcome.Set("unknown")
		mysqlDss.Duration.Count.Set(10)
		mysqlDss.Duration.Sum.Us.Set(durationSumUs)
		input.DroppedSpanStats = append(input.DroppedSpanStats, esDss, mysqlDss)

		mapToTransactionModel(&input, &out)
		expected := model.APMEvent{Transaction: &model.Transaction{
			DroppedSpansStats: []model.DroppedSpanStats{
				{
					DestinationServiceResource: "https://elasticsearch:9200",
					Outcome:                    "success",
					Duration: model.AggregatedDuration{
						Count: 2,
						Sum:   time.Duration(durationSumUs) * time.Microsecond,
					},
				},
				{
					DestinationServiceResource: "mysql://mysql:3306",
					Outcome:                    "unknown",
					Duration: model.AggregatedDuration{
						Count: 10,
						Sum:   time.Duration(durationSumUs) * time.Microsecond,
					},
				},
			},
		}}
		assert.Equal(t,
			expected.Transaction.DroppedSpansStats,
			out.Transaction.DroppedSpansStats,
		)
	})

	t.Run("client-ip-header", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		// from headers (case insensitive)
		input.Context.Request.Headers.Val.Add("x-Real-ip", gatewayIP.String())
		mapToTransactionModel(&input, &out)
		assert.Equal(t, gatewayIP.String(), out.Client.IP.String())
		// ignore if set in event already
		out = model.APMEvent{
			Client: model.Client{IP: net.ParseIP("192.17.1.1")},
		}
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "192.17.1.1", out.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		// set invalid headers
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-Real-ip", "192.13.14:8097")
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&input, &out)
		// ensure client ip is populated from socket
		assert.Equal(t, randomIP.String(), out.Client.IP.String())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "test@user.com", out.User.Email)
		assert.Zero(t, out.User.ID)
		assert.Zero(t, out.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// All the below exceptions are tested separately
			switch key {
			case
				// Tested separately
				"RepresentativeCount",
				// Kind is tested further down
				"Kind",

				// Not set for transaction events, tested in metricset decoding:
				"AggregatedDuration",
				"AggregatedDuration.Count",
				"AggregatedDuration.Sum",
				"DurationHistogram",
				"DurationHistogram.Counts",
				"DurationHistogram.Values",
				"Root":
				return true
			}
			// Tested separately
			return strings.HasPrefix(key, "DroppedSpansStats")
		}

		var input transaction
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.OTel.Reset()
		mapToTransactionModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)

		// leave event timestamp unmodified if eventTime is zero
		out1.Timestamp = reqTime
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		input.OTel.Reset()
		mapToTransactionModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Transaction, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input transaction
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b"}, "c": []string{"d", "e"}}, out.HTTP.Request.Headers)
		assert.Equal(t, common.MapStr{"f": []string{"g"}}, out.HTTP.Response.Headers)
	})

	t.Run("http-request-body", func(t *testing.T) {
		var input transaction
		input.Context.Request.Body.Set(map[string]interface{}{
			"a": json.Number("123.456"),
			"b": nil,
			"c": "d",
		})
		var out model.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, map[string]interface{}{"a": 123.456, "c": "d"}, out.HTTP.Request.Body)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		input.Context.Page.URL.Set("https://my.site.test:9201")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.URL.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.HTTP.Request.Referrer)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.OTel.Reset()
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, 4.0, out.Transaction.RepresentativeCount)
		// sample rate is not set -> Representative Count should be 1 by default
		out.Transaction.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, 1.0, out.Transaction.RepresentativeCount)
		// sample rate is set to 0
		out.Transaction.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Set(0)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, 0.0, out.Transaction.RepresentativeCount)
	})

	t.Run("outcome", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from other fields - success
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
		// derive from other fields - failure
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusInternalServerError)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from other fields - unknown
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
	})

	t.Run("session", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.Session.ID.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, model.Session{}, out.Session)

		input.Session.ID.Set("session_id")
		input.Session.Sequence.Set(123)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, model.Session{
			ID:       "session_id",
			Sequence: 123,
		}, out.Session)
	})

	// OTel bridge tests are mostly testing that the TranslateTransaction
	// function is being called, with more rigorous testing taking place
	// with that package.
	t.Run("otel-bridge", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			expected := model.URL{
				Scheme:   "https",
				Original: "/foo?bar",
				Full:     "https://testing.invalid:80/foo?bar",
				Path:     "/foo",
				Query:    "bar",
				Domain:   "testing.invalid",
				Port:     80,
			}
			attrs := map[string]interface{}{
				"http.scheme":   "https",
				"net.host.name": "testing.invalid",
				"net.host.port": json.Number("80"),
				"http.target":   "/foo?bar",
			}
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, expected, event.URL)
			assert.Equal(t, "SERVER", event.Span.Kind)
		})

		t.Run("net", func(t *testing.T) {
			expectedDomain := "source.domain"
			expectedIP := "192.168.0.1"
			expectedPort := 1234
			attrs := map[string]interface{}{
				"net.peer.name": "source.domain",
				"net.peer.ip":   "192.168.0.1",
				"net.peer.port": json.Number("1234"),
			}

			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			mapToTransactionModel(&input, &event)

			require.NotNil(t, event.HTTP)
			require.NotNil(t, event.HTTP.Request)
			parsedIP := net.ParseIP(expectedIP)
			require.NotNil(t, parsedIP)
			assert.Equal(t, model.Source{
				Domain: expectedDomain,
				IP:     net.ParseIP(expectedIP),
				Port:   expectedPort,
			}, event.Source)
			assert.Equal(t, model.Client(event.Source), event.Client)
			assert.Equal(t, "INTERNAL", event.Span.Kind)
		})

		t.Run("rpc", func(t *testing.T) {
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())

			attrs := map[string]interface{}{
				"rpc.system":           "grpc",
				"rpc.service":          "myservice.EchoService",
				"rpc.method":           "exampleMethod",
				"rpc.grpc.status_code": json.Number(strconv.Itoa(int(codes.Unavailable))),
				"net.peer.name":        "peer_name",
				"net.peer.ip":          "10.20.30.40",
				"net.peer.port":        json.Number("123"),
			}
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "request", event.Transaction.Type)
			assert.Equal(t, "Unavailable", event.Transaction.Result)
			assert.Equal(t, model.Client{
				Domain: "peer_name",
				IP:     net.ParseIP("10.20.30.40"),
				Port:   123,
			}, event.Client)
			assert.Equal(t, "SERVER", event.Span.Kind)
		})

		t.Run("messaging", func(t *testing.T) {
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.Type.Reset()
			attrs := map[string]interface{}{
				"message_bus.destination": "myQueue",
			}
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "messaging", event.Transaction.Type)
			assert.Equal(t, "CONSUMER", event.Span.Kind)
			assert.Equal(t, &model.Message{
				QueueName: "myQueue",
			}, event.Transaction.Message)
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
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			mapToTransactionModel(&input, &event)

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

		t.Run("double_attr", func(t *testing.T) {
			attrs := map[string]interface{}{
				"double_attr": json.Number("123.456"),
			}
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.Context.Tags = make(common.MapStr)
			input.OTel.Attributes = attrs
			input.Type.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, common.MapStr{}, event.Labels)
			assert.Equal(t, common.MapStr{"double_attr": 123.456}, event.NumericLabels)
		})

		t.Run("kind", func(t *testing.T) {
			var input transaction
			var event model.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.SpanKind.Set("CLIENT")

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "CLIENT", event.Span.Kind)
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
