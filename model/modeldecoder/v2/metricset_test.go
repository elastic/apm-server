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
		input := modeldecoder.Input{Base: eventBase, Config: modeldecoder.Config{}}
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
	metadataExceptions := func(key string) bool {
		// metadata are tested separately
		if strings.HasPrefix(key, "Metadata") ||
			// only set by aggregator
			strings.HasPrefix(key, "Event") ||
			key == "DocCount" ||
			key == "Name" ||
			key == "TimeseriesInstanceID" ||
			key == "Transaction.Result" ||
			key == "Transaction.Root" ||
			strings.HasPrefix(key, "Span.DestinationService") ||
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

		mapToMetricsetModel(&input, modeldecoder.Config{}, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Metricset, metadataExceptions, defaultVal)
		defaultSamples := map[string]model.MetricsetSample{
			defaultVal.Str + "0": {
				Type:   model.MetricType(defaultVal.Str),
				Unit:   defaultVal.Str,
				Value:  defaultVal.Float,
				Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
				Values: repeatFloat64(defaultVal.Float, defaultVal.N),
			},
			defaultVal.Str + "1": {
				Type:   model.MetricType(defaultVal.Str),
				Unit:   defaultVal.Str,
				Value:  defaultVal.Float,
				Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
				Values: repeatFloat64(defaultVal.Float, defaultVal.N),
			},
			defaultVal.Str + "2": {
				Type:   model.MetricType(defaultVal.Str),
				Unit:   defaultVal.Str,
				Value:  defaultVal.Float,
				Counts: repeatInt64(int64(defaultVal.Int), defaultVal.N),
				Values: repeatFloat64(defaultVal.Float, defaultVal.N),
			},
		}
		assert.Equal(t, defaultSamples, out1.Metricset.Samples)

		// leave Timestamp unmodified if eventTime is zero
		out1.Timestamp = now
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToMetricsetModel(&input, modeldecoder.Config{}, &out1)
		defaultVal.Update(now)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Metricset, metadataExceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, modeldecoder.Config{}, &out2)
		modeldecodertest.AssertStructValues(t, out2.Metricset, metadataExceptions, otherVal)
		otherSamples := map[string]model.MetricsetSample{
			otherVal.Str + "0": {
				Type:   model.MetricType(otherVal.Str),
				Unit:   otherVal.Str,
				Value:  otherVal.Float,
				Counts: repeatInt64(int64(otherVal.Int), otherVal.N),
				Values: repeatFloat64(otherVal.Float, otherVal.N),
			},
			otherVal.Str + "1": {
				Type:   model.MetricType(otherVal.Str),
				Unit:   otherVal.Str,
				Value:  otherVal.Float,
				Counts: repeatInt64(int64(otherVal.Int), otherVal.N),
				Values: repeatFloat64(otherVal.Float, otherVal.N),
			},
		}
		assert.Equal(t, otherSamples, out2.Metricset.Samples)
		modeldecodertest.AssertStructValues(t, out1.Metricset, metadataExceptions, defaultVal)
		assert.Equal(t, defaultSamples, out1.Metricset.Samples)
	})
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
