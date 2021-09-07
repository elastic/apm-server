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

package rumv3

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestResetTransactionOnRelease(t *testing.T) {
	inp := `{"x":{"n":"tr-a"}}`
	tr := fetchTransactionRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(tr))
	require.True(t, tr.IsSet())
	releaseTransactionRoot(tr)
	assert.False(t, tr.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		eventBase := initializedMetadata()
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"x":{"n":"tr-a","d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"y":[{"n":"a","d":10,"t":"http","id":"123","s":20}],"me":[{"sa":{"xds":{"v":2048}}},{"sa":{"ysc":{"v":5}},"y":{"t":"span_type","su":"span_subtype"}}]}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		require.Len(t, batch, 4) // 1 transaction, 2 metricsets, 1 span
		require.NotNil(t, batch[0].Transaction)
		require.NotNil(t, batch[1].Metricset)
		require.NotNil(t, batch[2].Metricset)
		require.NotNil(t, batch[3].Span)

		assert.Equal(t, "request", batch[0].Transaction.Type)
		// fall back to request time
		assert.Equal(t, now, batch[0].Timestamp)

		// Ensure nested metricsets are decoded. RUMv3 only sends
		// breakdown metrics, so the Metricsets will be empty and
		// metrics will be recorded on the Transaction and Span
		// fields.
		assert.Equal(t, &model.Metricset{}, batch[1].Metricset)
		assert.Equal(t, &model.Transaction{
			Name:               "tr-a",
			Type:               "request",
			AggregatedDuration: model.AggregatedDuration{Sum: 2048 * time.Microsecond},
		}, batch[1].Transaction)
		assert.Equal(t, &model.Metricset{}, batch[2].Metricset)
		assert.Equal(t, &model.Transaction{
			Name: "tr-a",
			Type: "request",
		}, batch[2].Transaction)
		assert.Equal(t, &model.Span{
			Type:     "span_type",
			Subtype:  "span_subtype",
			SelfTime: model.AggregatedDuration{Count: 5},
		}, batch[2].Span)
		assert.Equal(t, now, batch[2].Timestamp)

		// ensure nested spans are decoded
		start := time.Duration(20 * 1000 * 1000)
		assert.Equal(t, now.Add(start), batch[3].Timestamp) //add start to timestamp
		assert.Equal(t, "100", batch[3].Transaction.ID)
		assert.Equal(t, "1", batch[3].Trace.ID)
		assert.Equal(t, "100", batch[3].Parent.ID)

		for _, event := range batch {
			modeldecodertest.AssertStructValues(
				t, &event,
				metadataExceptions("Timestamp"), // timestamp checked above
				modeldecodertest.DefaultValues(),
			)
		}

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("decode-marks", func(t *testing.T) {
		now := time.Now()
		eventBase := model.APMEvent{Timestamp: now}
		input := modeldecoder.Input{Base: eventBase}
		str := `{"x":{"d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"k":{"a":{"dc":0.1,"di":0.2,"ds":0.3,"de":0.4,"fb":0.5,"fp":0.6,"lp":0.7,"long":0.8},"nt":{"fs":0.1,"ls":0.2,"le":0.3,"cs":0.4,"ce":0.5,"qs":0.6,"rs":0.7,"re":0.8,"dl":0.9,"di":0.11,"ds":0.21,"de":0.31,"dc":0.41,"es":0.51,"ee":6,"long":0.99},"long":{"long":0.1}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		marks := model.TransactionMarks{
			"agent": map[string]float64{
				"domComplete":                0.1,
				"domInteractive":             0.2,
				"domContentLoadedEventStart": 0.3,
				"domContentLoadedEventEnd":   0.4,
				"timeToFirstByte":            0.5,
				"firstContentfulPaint":       0.6,
				"largestContentfulPaint":     0.7,
				"long":                       0.8,
			},
			"navigationTiming": map[string]float64{
				"fetchStart":                 0.1,
				"domainLookupStart":          0.2,
				"domainLookupEnd":            0.3,
				"connectStart":               0.4,
				"connectEnd":                 0.5,
				"requestStart":               0.6,
				"responseStart":              0.7,
				"responseEnd":                0.8,
				"domLoading":                 0.9,
				"domInteractive":             0.11,
				"domContentLoadedEventStart": 0.21,
				"domContentLoadedEventEnd":   0.31,
				"domComplete":                0.41,
				"loadEventStart":             0.51,
				"loadEventEnd":               6,
				"long":                       0.99,
			},
			"long": map[string]float64{
				"long": 0.1,
			},
		}
		assert.Equal(t, marks, batch[0].Transaction.Marks)
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	localhostIP := net.ParseIP("127.0.0.1")

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var input transaction
		out := initializedMetadata()
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, &out)

		// user-agent should be set to context request header values
		assert.Equal(t, "d, e", out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		assert.Equal(t, localhostIP, out.Client.IP, out.Client.IP.String())
		assert.Equal(t, common.MapStr{
			"init0": "init", "init1": "init", "init2": "init",
			"overwritten0": "overwritten", "overwritten1": "overwritten",
		}, out.Labels)
		// service values should be set
		modeldecodertest.AssertStructValues(t, &out.Service, metadataExceptions("Node", "Agent.EphemeralID"), otherVal)
		// user values should be set
		modeldecodertest.AssertStructValues(t, &out.User, metadataExceptions(), otherVal)
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		var out model.APMEvent
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "test@user.com", out.User.Email)
		assert.Zero(t, out.User.ID)
		assert.Zero(t, out.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// values not set for RUM v3
				"HTTP.Request.Env", "HTTP.Request.Body", "HTTP.Request.Socket", "HTTP.Request.Cookies",
				"HTTP.Response.HeadersSent", "HTTP.Response.Finished",
				"Experimental",
				"RepresentativeCount", "Root", "Message",
				// HTTP headers tested separately
				"HTTP.Request.Headers",
				"HTTP.Response.Headers",
				// URL parts are derived from page.url (separately tested)
				"URL", "Page.URL",
				// Not set for transaction events:
				"AggregatedDuration",
				"AggregatedDuration.Count",
				"AggregatedDuration.Sum",
				"BreakdownCount",
				"DurationHistogram",
				"DurationHistogram.Counts",
				"DurationHistogram.Values",
				"Root",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input transaction
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, &out1)
		input.Reset()
		defaultVal.Update(reqTime) //for rumv3 the timestamp is always set from the base event
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		otherVal.Update(reqTime) //for rumv3 the timestamp is always set from the base event
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Transaction, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// values not set for RUM v3
				"ChildIDs",
				"Composite",
				"DB",
				"Experimental",
				"HTTP.Response.Headers",
				"Message",
				"RepresentativeCount",
				"Stacktrace.LibraryFrame",
				"Stacktrace.Vars",
				// set as HTTP.StatusCode for RUM v3
				"HTTP.Response.StatusCode",
				// Not set for HTTP spans
				"HTTP.Request.Env", "HTTP.Request.Body", "HTTP.Request.Socket", "HTTP.Request.Cookies",
				"HTTP.Response.HeadersSent", "HTTP.Response.Finished",
				"HTTP.Request.Body", "HTTP.Request.Headers", "HTTP.Response.Headers", "HTTP.Request.Referrer",
				"HTTP.Version",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"Stacktrace.Original",
				"Stacktrace.Sourcemap",
				// ExcludeFromGrouping is set when processing the event
				"Stacktrace.ExcludeFromGrouping",
				// Not set for span events:
				"DestinationService.ResponseTime",
				"DestinationService.ResponseTime.Count",
				"DestinationService.ResponseTime.Sum",
				"SelfTime",
				"SelfTime.Count",
				"SelfTime.Sum",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input span
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToSpanModel(&input, &out1)
		input.Reset()
		defaultStart := time.Duration(defaultVal.Float * 1000 * 1000)
		defaultVal.Update(reqTime.Add(defaultStart)) //for rumv3 the timestamp is always set from the base event
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToSpanModel(&input, &out2)
		otherStart := time.Duration(otherVal.Float * 1000 * 1000)
		otherVal.Update(reqTime.Add(otherStart)) //for rumv3 the timestamp is always set from the base event
		modeldecodertest.AssertStructValues(t, out2.Span, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)
	})

	t.Run("span-outcome", func(t *testing.T) {
		var input span
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.HTTP.StatusCode.Set(http.StatusPermanentRedirect)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from span fields - success
		input.Outcome.Reset()
		input.Context.HTTP.StatusCode.Set(http.StatusPermanentRedirect)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
		// derive from span fields - failure
		input.Outcome.Reset()
		input.Context.HTTP.StatusCode.Set(http.StatusBadRequest)
		mapToSpanModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from span fields - unknown
		input.Outcome.Reset()
		input.Context.HTTP.StatusCode.Reset()
		mapToSpanModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
	})

	t.Run("transaction-outcome", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from span fields - success
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "success", out.Event.Outcome)
		// derive from span fields - failure
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusInternalServerError)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "failure", out.Event.Outcome)
		// derive from span fields - unknown
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "unknown", out.Event.Outcome)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.URL.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input transaction
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Transaction.HTTP.Request.Referrer)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input transaction
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b"}, "c": []string{"d", "e"}}, out.Transaction.HTTP.Request.Headers)
		assert.Equal(t, common.MapStr{"f": []string{"g"}}, out.Transaction.HTTP.Response.Headers)
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
}
