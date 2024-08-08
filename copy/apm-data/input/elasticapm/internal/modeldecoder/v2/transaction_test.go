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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/nullable"
	"github.com/elastic/apm-data/model/modelpb"
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
		now := modelpb.FromTime(time.Now())
		input := modeldecoder.Input{Base: &modelpb.APMEvent{}}
		str := `{"transaction":{"duration":100,"timestamp":1599996822281000,"id":"100","trace_id":"1","type":"request","span_count":{"started":2}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))

		var batch modelpb.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Transaction)
		assert.Equal(t, "request", batch[0].Transaction.Type)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", modelpb.ToTime(batch[0].Timestamp).String())

		input = modeldecoder.Input{Base: &modelpb.APMEvent{Timestamp: now}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = modelpb.Batch{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		// if no timestamp is provided, fall back to base event timestamp
		assert.Equal(t, now, batch[0].Timestamp)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	gatewayIP := modelpb.MustParseIP("192.168.0.1")
	randomIP := modelpb.MustParseIP("71.0.54.1")

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input transaction
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.GetClient().GetIp())
		assert.Equal(t, modelpb.Labels{
			"init0": {Global: true, Value: "init"}, "init1": {Global: true, Value: "init"}, "init2": {Global: true, Value: "init"},
			"overwritten0": {Value: "overwritten"}, "overwritten1": {Value: "overwritten"},
		}, modelpb.Labels(out.Labels))
		//assert.Equal(t, tLabels, out.Transaction.Labels)
		exceptions := func(key string) bool { return false }
		modeldecodertest.AssertStructValues(t, &out.Service, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out.User, exceptions, otherVal)
	})

	t.Run("cloud.origin", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		origin := contextCloudOrigin{}
		origin.Account.ID.Set("accountID")
		origin.Provider.Set("aws")
		origin.Region.Set("us-east-1")
		origin.Service.Name.Set("serviceName")
		input.Context.Cloud.Origin = origin
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "accountID", out.Cloud.Origin.AccountId)
		assert.Equal(t, "aws", out.Cloud.Origin.Provider)
		assert.Equal(t, "us-east-1", out.Cloud.Origin.Region)
		assert.Equal(t, "serviceName", out.Cloud.Origin.ServiceName)
	})

	t.Run("service.origin", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		origin := contextServiceOrigin{}
		origin.ID.Set("abc123")
		origin.Name.Set("name")
		origin.Version.Set("1.0")
		input.Context.Service.Origin = origin
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "abc123", out.Service.Origin.Id)
		assert.Equal(t, "name", out.Service.Origin.Name)
		assert.Equal(t, "1.0", out.Service.Origin.Version)
	})

	t.Run("service.target", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		target := contextServiceTarget{}
		target.Name.Set("testdb")
		target.Type.Set("oracle")
		input.Context.Service.Target = target
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "testdb", out.Service.Target.Name)
		assert.Equal(t, "oracle", out.Service.Target.Type)
	})

	t.Run("faas", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		input.FAAS.ID.Set("faasID")
		input.FAAS.Coldstart.Set(true)
		input.FAAS.Execution.Set("execution")
		input.FAAS.Trigger.Type.Set("http")
		input.FAAS.Trigger.RequestID.Set("abc123")
		input.FAAS.Name.Set("faasName")
		input.FAAS.Version.Set("1.0.0")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "faasID", out.Faas.Id)
		assert.True(t, *out.Faas.ColdStart)
		assert.Equal(t, "execution", out.Faas.Execution)
		assert.Equal(t, "http", out.Faas.TriggerType)
		assert.Equal(t, "abc123", out.Faas.TriggerRequestId)
		assert.Equal(t, "faasName", out.Faas.Name)
		assert.Equal(t, "1.0.0", out.Faas.Version)
	})

	t.Run("dropped_span_stats", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
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
		expected := modelpb.APMEvent{Transaction: &modelpb.Transaction{
			DroppedSpansStats: []*modelpb.DroppedSpanStats{
				{
					DestinationServiceResource: "https://elasticsearch:9200",
					Outcome:                    "success",
					Duration: &modelpb.AggregatedDuration{
						Count: 2,
						Sum:   uint64(time.Duration(durationSumUs) * time.Microsecond),
					},
				},
				{
					DestinationServiceResource: "mysql://mysql:3306",
					Outcome:                    "unknown",
					Duration: &modelpb.AggregatedDuration{
						Count: 10,
						Sum:   uint64(time.Duration(durationSumUs) * time.Microsecond),
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
		var out modelpb.APMEvent
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Socket.RemoteAddress.Set(modelpb.IP2String(randomIP))
		// from headers (case insensitive)
		input.Context.Request.Headers.Val.Add("x-Real-ip", modelpb.IP2String(gatewayIP))
		mapToTransactionModel(&input, &out)
		assert.Equal(t, gatewayIP, out.GetClient().GetIp())
		// ignore if set in event already
		out = modelpb.APMEvent{
			Client: &modelpb.Client{Ip: modelpb.MustParseIP("192.17.1.1")},
		}
		mapToTransactionModel(&input, &out)
		assert.Equal(t, modelpb.MustParseIP("192.17.1.1"), out.Client.Ip)
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		// set invalid headers
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-Real-ip", "192.13.14:8097")
		input.Context.Request.Socket.RemoteAddress.Set(modelpb.IP2String(randomIP))
		mapToTransactionModel(&input, &out)
		// ensure client ip is populated from socket
		assert.Equal(t, randomIP, out.GetClient().GetIp())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, out)
		assert.Equal(t, "test@user.com", out.User.Email)
		assert.Zero(t, out.User.Id)
		assert.Zero(t, out.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// All the below exceptions are tested separately
			switch key {
			case
				// Tested separately
				"representative_count",
				// Kind is tested further down
				"Kind",
				// profiler_stack_trace_ids are supplied as OTel-attributes
				"profiler_stack_trace_ids",

				// Not set for transaction events, tested in metricset decoding:
				"AggregatedDuration",
				"AggregatedDuration.Count",
				"AggregatedDuration.Sum",
				"duration_histogram",
				"DurationHistogram.Counts",
				"DurationHistogram.Values",
				"duration_summary",
				"DurationSummary.Count",
				"DurationSummary.Sum",
				"message.headers.key",
				"message.headers.value",
				"FailureCount",
				"SuccessCount",
				"SuccessCount.Count",
				"SuccessCount.Sum",
				"root":
				return true
			}
			// Tested separately
			return strings.HasPrefix(key, "dropped_spans_stats")
		}

		var input transaction
		var out1, out2 modelpb.APMEvent
		reqTime := modelpb.FromTime(time.Now().Add(time.Second))
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.OTel.Reset()
		mapToTransactionModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)

		// leave base event timestamp unmodified if event timestamp is unspecified
		out1.Timestamp = reqTime
		mapToTransactionModel(&input, &out1)
		assert.Equal(t, reqTime, out1.Timestamp)
		input.Reset()

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, &out1)
		input.Reset()
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
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Empty(t, cmp.Diff([]*modelpb.HTTPHeader{
			{
				Key:   "a",
				Value: []string{"b"},
			},
			{
				Key:   "c",
				Value: []string{"d", "e"},
			},
		}, out.Http.Request.Headers,
			cmpopts.SortSlices(func(x, y *modelpb.HTTPHeader) bool {
				return x.Key < y.Key
			}),
			protocmp.Transform(),
		))
		assert.Empty(t, cmp.Diff([]*modelpb.HTTPHeader{
			{
				Key:   "f",
				Value: []string{"g"},
			},
		}, out.Http.Response.Headers, protocmp.Transform()))
	})

	t.Run("http-request-body", func(t *testing.T) {
		var input transaction
		input.Context.Request.Body.Set(map[string]interface{}{
			"a": json.Number("123.456"),
			"b": nil,
			"c": "d",
		})
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
		requestBody, err := structpb.NewValue(map[string]interface{}{"a": 123.456, "c": "d"})
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(requestBody, out.Http.Request.Body, protocmp.Transform()))
	})

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		input.Context.Page.URL.Set("https://my.site.test:9201")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Url.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Http.Request.Referrer)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
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
		var out modelpb.APMEvent
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
		input.OTel.Reset()
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
		// outcome is success when not assigned and it's otel
		input.Outcome.Reset()
		input.OTel.SpanKind.Set(spanKindInternal)
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
	})

	t.Run("session", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.Session.ID.Reset()
		mapToTransactionModel(&input, &out)
		assert.Nil(t, out.Session)

		input.Session.ID.Set("session_id")
		input.Session.Sequence.Set(123)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, &modelpb.Session{
			Id:       "session_id",
			Sequence: 123,
		}, out.Session)
	})

	// OTel bridge tests are mostly testing that the TranslateTransaction
	// function is being called, with more rigorous testing taking place
	// with that package.
	t.Run("otel-bridge", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			expected := modelpb.URL{
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
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			input.Type.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, &expected, event.Url)
			assert.Equal(t, "SERVER", event.Span.Kind)
		})

		t.Run("net", func(t *testing.T) {
			expectedDomain := "source.domain"
			expectedIP := modelpb.MustParseIP("192.168.0.1")
			expectedPort := 1234
			attrs := map[string]interface{}{
				"net.peer.name": "source.domain",
				"net.peer.ip":   "192.168.0.1",
				"net.peer.port": json.Number("1234"),
			}

			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			mapToTransactionModel(&input, &event)

			require.NotNil(t, event.Http)
			require.NotNil(t, event.Http.Request)
			assert.Equal(t, &modelpb.Source{
				Domain: expectedDomain,
				Ip:     expectedIP,
				Port:   uint32(expectedPort),
			}, event.Source)
			want := modelpb.Client{Ip: event.Source.Ip, Port: event.Source.Port, Domain: event.Source.Domain}
			assert.Equal(t, &want, event.Client)
			assert.Equal(t, "INTERNAL", event.Span.Kind)
		})

		t.Run("rpc", func(t *testing.T) {
			var input transaction
			var event modelpb.APMEvent
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
			input.Type.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "request", event.Transaction.Type)
			assert.Equal(t, "Unavailable", event.Transaction.Result)
			assert.Equal(t, &modelpb.Client{
				Domain: "peer_name",
				Ip:     modelpb.MustParseIP("10.20.30.40"),
				Port:   123,
			}, event.Client)
			assert.Equal(t, "SERVER", event.Span.Kind)
		})

		t.Run("messaging", func(t *testing.T) {
			var input transaction
			var event modelpb.APMEvent
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
			assert.Equal(t, &modelpb.Message{
				QueueName: "myQueue",
			}, event.Transaction.Message)
		})

		t.Run("messaging_without_destination", func(t *testing.T) {
			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.Type.Reset()
			attrs := map[string]interface{}{
				"messaging.system":    "kafka",
				"messaging.operation": "publish",
			}
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "messaging", event.Transaction.Type)
			assert.Equal(t, "CONSUMER", event.Span.Kind)
			assert.Nil(t, event.Transaction.Message)
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
			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.Attributes = attrs
			input.OTel.SpanKind.Reset()
			mapToTransactionModel(&input, &event)

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
			assert.Equal(t, &expected, event.Network)
			assert.Equal(t, "INTERNAL", event.Span.Kind)
		})

		t.Run("double_attr", func(t *testing.T) {
			attrs := map[string]interface{}{
				"double_attr": json.Number("123.456"),
			}
			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.Context.Tags = make(map[string]any)
			input.OTel.Attributes = attrs
			input.Type.Reset()

			mapToTransactionModel(&input, &event)
			assert.Equal(t, modelpb.Labels{}, modelpb.Labels(event.Labels))
			assert.Equal(t, modelpb.NumericLabels{"double_attr": {Value: 123.456}}, modelpb.NumericLabels(event.NumericLabels))
		})

		t.Run("kind", func(t *testing.T) {
			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			input.OTel.SpanKind.Set("CLIENT")

			mapToTransactionModel(&input, &event)
			assert.Equal(t, "CLIENT", event.Span.Kind)
		})

		t.Run("elastic-profiling-ids", func(t *testing.T) {
			var input transaction
			var event modelpb.APMEvent
			modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
			attrs := map[string]interface{}{
				"elastic.profiler_stack_trace_ids": []interface{}{"id1", "id2"},
			}
			input.OTel.Attributes = attrs

			mapToTransactionModel(&input, &event)
			assert.Equal(t, []string{"id1", "id2"}, event.Transaction.ProfilerStackTraceIds)
		})
	})
	t.Run("labels", func(t *testing.T) {
		var input transaction
		input.Context.Tags = map[string]any{
			"a": "b",
			"c": float64(12315124131),
			"d": 12315124131.12315124131,
			"e": true,
		}
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
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
		var input transaction
		input.Links = []spanLink{{
			SpanID:  nullable.String{Val: "span1"},
			TraceID: nullable.String{Val: "trace1"},
		}, {
			SpanID:  nullable.String{Val: "span2"},
			TraceID: nullable.String{Val: "trace2"},
		}}
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)

		assert.Equal(t, []*modelpb.SpanLink{{
			SpanId:  "span1",
			TraceId: "trace1",
		}, {
			SpanId:  "span2",
			TraceId: "trace2",
		}}, out.Span.Links)
	})
	t.Run("transaction.type", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, &modelpb.Transaction{
			Type:                "unknown",
			Sampled:             true,
			RepresentativeCount: 1,
		}, out.Transaction)
	})
	t.Run("span.type", func(t *testing.T) {
		var input span
		var out modelpb.APMEvent
		mapToSpanModel(&input, &out)
		assert.Equal(t, &modelpb.Span{
			Type:                "unknown",
			RepresentativeCount: 1,
		}, out.Span)
	})

	t.Run("transaction-span.id", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		input.ID.Set("1234")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "1234", out.Transaction.Id)
		assert.Equal(t, "1234", out.Span.Id)
	})
}
