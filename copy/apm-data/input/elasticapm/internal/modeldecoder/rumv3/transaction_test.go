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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-data/model/modelpb"
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
		now := modelpb.FromTime(time.Now())
		eventBase := initializedMetadata()
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"x":{"n":"tr-a","d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"y":[{"n":"a","d":10,"t":"http","id":"123","s":20}],"me":[{"sa":{"ysc":{"v":5}},"y":{"t":"span_type","su":"span_subtype"}}]}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch modelpb.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		require.Len(t, batch, 3) // 1 transaction, 1 metricset, 1 span
		require.NotNil(t, batch[0].Transaction)
		require.NotNil(t, batch[1].Metricset)
		require.NotNil(t, batch[2].Span)

		assert.Equal(t, "request", batch[0].Transaction.Type)
		// fall back to request time
		assert.Equal(t, now, batch[0].Timestamp)

		// Ensure nested metricsets are decoded. RUMv3 only sends
		// breakdown metrics, so the Metricsets will be empty and
		// metrics will be recorded on the Transaction and Span
		// fields.
		assert.Empty(t, cmp.Diff(&modelpb.Metricset{}, batch[1].Metricset, protocmp.Transform()))
		assert.Empty(t, cmp.Diff(&modelpb.Transaction{
			Name: "tr-a",
			Type: "request",
		}, batch[1].Transaction, protocmp.Transform()))
		assert.Empty(t, cmp.Diff(&modelpb.Span{
			Type:     "span_type",
			Subtype:  "span_subtype",
			SelfTime: &modelpb.AggregatedDuration{Count: 5},
		}, batch[1].Span, protocmp.Transform()))
		assert.Equal(t, now, batch[1].Timestamp)

		// ensure nested spans are decoded
		start := uint64(time.Duration(20 * 1000 * 1000).Nanoseconds())
		assert.Equal(t, now+start, batch[2].Timestamp) // add start to timestamp
		assert.Equal(t, "100", batch[2].Transaction.Id)
		assert.Equal(t, "1", batch[2].Trace.Id)
		assert.Equal(t, "100", batch[2].ParentId)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("decode-marks", func(t *testing.T) {
		now := modelpb.FromTime(time.Now())
		eventBase := modelpb.APMEvent{Timestamp: now}
		input := modeldecoder.Input{Base: &eventBase}
		str := `{"x":{"d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"k":{"a":{"dc":0.1,"di":0.2,"ds":0.3,"de":0.4,"fb":0.5,"fp":0.6,"lp":0.7,"long":0.8},"nt":{"fs":0.1,"ls":0.2,"le":0.3,"cs":0.4,"ce":0.5,"qs":0.6,"rs":0.7,"re":0.8,"dl":0.9,"di":0.11,"ds":0.21,"de":0.31,"dc":0.41,"es":0.51,"ee":6,"long":0.99},"long":{"long":0.1}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch modelpb.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		marks := map[string]*modelpb.TransactionMark{
			"agent": {
				Measurements: map[string]float64{
					"domComplete":                0.1,
					"domInteractive":             0.2,
					"domContentLoadedEventStart": 0.3,
					"domContentLoadedEventEnd":   0.4,
					"timeToFirstByte":            0.5,
					"firstContentfulPaint":       0.6,
					"largestContentfulPaint":     0.7,
					"long":                       0.8,
				},
			},
			"navigationTiming": {
				Measurements: map[string]float64{
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
			},
			"long": {
				Measurements: map[string]float64{
					"long": 0.1,
				},
			},
		}
		assert.Equal(t, marks, batch[0].Transaction.Marks)
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	localhostIP := modelpb.MustParseIP("127.0.0.1")

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var input transaction
		out := initializedMetadata()
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, out)

		// user-agent should be set to context request header values
		assert.Equal(t, "d, e", out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		assert.Equal(t, localhostIP, out.Client.Ip)
		assert.Equal(t, modelpb.Labels{
			"init0": {Global: true, Value: "init"}, "init1": {Global: true, Value: "init"}, "init2": {Global: true, Value: "init"},
			"overwritten0": {Value: "overwritten"}, "overwritten1": {Value: "overwritten"},
		}, modelpb.Labels(out.Labels))
		// service values should be set
		modeldecodertest.AssertStructValues(t, &out.Service, metadataExceptions("node", "Agent.EphemeralID"), otherVal)
		// user values should be set
		modeldecodertest.AssertStructValues(t, &out.User, metadataExceptions(), otherVal)
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		var out modelpb.APMEvent
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "test@user.com", out.User.Email)
		assert.Zero(t, out.User.Id)
		assert.Zero(t, out.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// values not set for RUM v3
				"Kind", "representative_count", "message", "dropped_spans_stats", "profiler_stack_trace_ids",
				// Not set for transaction events:
				"AggregatedDuration",
				"AggregatedDuration.Count",
				"AggregatedDuration.Sum",
				"BreakdownCount",
				"duration_histogram",
				"DurationHistogram.Counts",
				"DurationHistogram.Values",
				"duration_summary",
				"DurationSummary.Count",
				"DurationSummary.Sum",
				"FailureCount",
				"SuccessCount",
				"root",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input transaction
		var out1, out2 modelpb.APMEvent
		reqTime := modelpb.FromTime(time.Now().Add(time.Second))
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Transaction, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Transaction, exceptions, defaultVal)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// values not set for RUM v3
				"kind",
				"ChildIDs",
				"composite",
				"db",
				"message",
				"representative_count",
				"stacktrace.library_frame",
				"stacktrace.vars",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"stacktrace.original",
				"stacktrace.sourcemap",
				// ExcludeFromGrouping is set when processing the event
				"stacktrace.exclude_from_grouping",
				// Not set for span events:
				"links",
				"destination_service.response_time",
				"DestinationService.ResponseTime.Count",
				"DestinationService.ResponseTime.Sum",
				"self_time",
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
		var out1, out2 modelpb.APMEvent
		reqTime := modelpb.FromTime(time.Now().Add(time.Second))
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToSpanModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToSpanModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Span, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Span, exceptions, defaultVal)
	})

	t.Run("span-outcome", func(t *testing.T) {
		var input span
		var out modelpb.APMEvent
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
		var out modelpb.APMEvent
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
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Url.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input transaction
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out modelpb.APMEvent
		mapToTransactionModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Http.Request.Referrer)
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
		assert.Equal(t, []*modelpb.HTTPHeader{
			{
				Key:   "f",
				Value: []string{"g"},
			},
		}, out.Http.Response.Headers)
	})

	t.Run("session", func(t *testing.T) {
		var input transaction
		var out modelpb.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.Session.ID.Reset()
		mapToTransactionModel(&input, &out)
		assert.Empty(t, &modelpb.Session{}, out.Session)

		input.Session.ID.Set("session_id")
		input.Session.Sequence.Set(123)
		mapToTransactionModel(&input, &out)
		assert.Equal(t, &modelpb.Session{
			Id:       "session_id",
			Sequence: 123,
		}, out.Session)
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
			Type: "unknown",
		}, out.Span)
	})
}
