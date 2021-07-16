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
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{}}
		str := `{"metricset":{"timestamp":1599996822281000,"samples":{"a.b":{"value":2048}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Metricset
		require.NoError(t, DecodeNestedMetricset(dec, &input, &out))
		assert.Equal(t, map[string]model.MetricsetSample{"a.b": {Value: 2048}}, out.Samples)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		// invalid type
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out model.Metricset
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToMetricsetModel(t *testing.T) {
	exceptions := func(key string) bool { return strings.HasPrefix(key, "System.Network") }

	t.Run("metadata-set", func(t *testing.T) {
		// set metadata - metricsets do not hold metadata themselves
		var input metricset
		var out model.Metricset
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		// iterate through metadata model and assert values are set to default values
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, modeldecodertest.DefaultValues())
	})

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
		var out1, out2 model.Metricset
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)

		mapToMetricsetModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{}, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, metadataExceptions, defaultVal)
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
		assert.Equal(t, defaultSamples, out1.Samples)

		// set Timestamp to requestTime if eventTime is zero
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToMetricsetModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{}, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, metadataExceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{}, &out2)
		modeldecodertest.AssertStructValues(t, &out2, metadataExceptions, otherVal)
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
		assert.Equal(t, otherSamples, out2.Samples)
		modeldecodertest.AssertStructValues(t, &out1, metadataExceptions, defaultVal)
		assert.Equal(t, defaultSamples, out1.Samples)
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
