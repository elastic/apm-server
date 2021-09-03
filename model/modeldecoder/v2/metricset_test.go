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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestResetMetricsetOnRelease(t *testing.T) {
	inp := `{"metricset":{"samples":{"a.b.":{"value":2048}}}}`
	root := fetchMetricsetRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseMetricsetRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedMetricset(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		defaultVal := modeldecodertest.DefaultValues()
		_, eventBase := initializedInputMetadata(defaultVal)
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"metricset":{"timestamp":1599996822281000,"samples":{"a.b":{"value":2048}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedMetricset(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Metricset)
		assert.Equal(t, map[string]model.MetricsetSample{"a.b": {Value: 2048}}, batch[0].Metricset.Samples)
		defaultVal.Update(time.Unix(1599996822, 281000000).UTC())
		modeldecodertest.AssertStructValues(t, &batch[0], isMetadataException, defaultVal)

		// invalid type
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToMetricsetModel(t *testing.T) {
	exceptions := func(key string) bool {
		if key == "DocCount" ||
			key == "Name" ||
			key == "TimeseriesInstanceID" ||
			// test Samples separately
			strings.HasPrefix(key, "Samples") {
			return true
		}
		return false
	}

	t.Run("metricset-values", func(t *testing.T) {
		var input metricset
		var out1, out2 model.APMEvent
		now := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Transaction.Reset() // tested by TestDecodeMetricsetInternal

		mapToMetricsetModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		defaultSamples := map[string]model.MetricsetSample{
			defaultVal.Str + "0": {
				Type:  model.MetricType(defaultVal.Str),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: model.Histogram{
					Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
			defaultVal.Str + "1": {
				Type:  model.MetricType(defaultVal.Str),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: model.Histogram{
					Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
			defaultVal.Str + "2": {
				Type:  model.MetricType(defaultVal.Str),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: model.Histogram{
					Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
		}
		assert.Equal(t, defaultSamples, out1.Metricset.Samples)

		// leave Timestamp unmodified if eventTime is zero
		out1.Timestamp = now
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Transaction.Reset()
		mapToMetricsetModel(&input, &out1)
		defaultVal.Update(now)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		input.Transaction.Reset()
		mapToMetricsetModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Metricset, exceptions, otherVal)
		otherSamples := map[string]model.MetricsetSample{
			otherVal.Str + "0": {
				Type:  model.MetricType(otherVal.Str),
				Unit:  otherVal.Str,
				Value: otherVal.Float,
				Histogram: model.Histogram{
					Counts: repeatInt64(int64(otherVal.Int), otherVal.N),
					Values: repeatFloat64(otherVal.Float, otherVal.N),
				},
			},
			otherVal.Str + "1": {
				Type:  model.MetricType(otherVal.Str),
				Unit:  otherVal.Str,
				Value: otherVal.Float,
				Histogram: model.Histogram{
					Counts: repeatInt64(int64(otherVal.Int), otherVal.N),
					Values: repeatFloat64(otherVal.Float, otherVal.N),
				},
			},
		}
		assert.Equal(t, otherSamples, out2.Metricset.Samples)
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		assert.Equal(t, defaultSamples, out1.Metricset.Samples)
	})
}

func TestDecodeMetricsetInternal(t *testing.T) {
	var batch model.Batch

	err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"transaction.breakdown.count": {"value": 123},
				"transaction.duration.count": {"value": 456},
				"transaction.duration.sum.us": {"value": 789}
			},
			"transaction": {
				"name": "transaction_name",
				"type": "transaction_type"
			}
		}
	}`)), &modeldecoder.Input{}, &batch)
	require.NoError(t, err)

	err = DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"span.self_time.count": {"value": 123},
				"span.self_time.sum.us": {"value": 456}
			},
			"transaction": {
				"name": "transaction_name",
				"type": "transaction_type"
			},
			"span": {
				"type": "span_type",
				"subtype": "span_subtype"
			}
		}
	}`)), &modeldecoder.Input{}, &batch)
	require.NoError(t, err)

	assert.Equal(t, model.Batch{{
		Timestamp: time.Unix(0, 0).UTC(),
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{},
		Transaction: &model.Transaction{
			Name:           "transaction_name",
			Type:           "transaction_type",
			BreakdownCount: 123,
			AggregatedDuration: model.AggregatedDuration{
				Count: 456,
				Sum:   789 * time.Microsecond,
			},
		},
	}, {
		Timestamp: time.Unix(0, 0).UTC(),
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{},
		Transaction: &model.Transaction{
			Name: "transaction_name",
			Type: "transaction_type",
		},
		Span: &model.Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: model.AggregatedDuration{
				Count: 123,
				Sum:   456 * time.Microsecond,
			},
		},
	}}, batch)
}

func repeatInt64(v int64, n int) []int64 {
	vs := make([]int64, n)
	for i := range vs {
		vs[i] = v
	}
	return vs
}

func repeatFloat64(v float64, n int) []float64 {
	vs := make([]float64, n)
	for i := range vs {
		vs[i] = v
	}
	return vs
}
