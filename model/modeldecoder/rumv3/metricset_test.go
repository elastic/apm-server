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
	inp := `{"me":{"sa":{"xdc":{"v":2048}}}}`
	root := fetchMetricsetRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseMetricsetRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedMetricset(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now}
		str := `{"me":{"sa":{"xds":{"v":2048}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Metricset
		require.NoError(t, DecodeNestedMetricset(dec, &input, &out))
		assert.Equal(t, map[string]model.MetricsetSample{"transaction.duration.sum.us": {Value: 2048}}, out.Samples)
		assert.Equal(t, now, out.Timestamp)

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
	t.Run("metadata-set", func(t *testing.T) {
		// set metadata - metricsets do not hold metadata themselves
		var input metricset
		var out model.Metricset
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, initializedMetadata(), time.Now(), &out)
		// iterate through metadata model and assert values are set to default values
		modeldecodertest.AssertStructValues(t, &out.Metadata, metadataExceptions(), modeldecodertest.DefaultValues())
	})

	t.Run("metricset-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// metadata are tested separately
			if strings.HasPrefix(key, "Metadata") ||
				// transaction is only set when metricset is nested inside transaction
				// tested within transaction tests
				strings.HasPrefix(key, "Transaction") ||
				// only set by aggregator
				strings.HasPrefix(key, "Event") ||
				key == "DocCount" ||
				key == "Name" ||
				key == "TimeseriesInstanceID" ||
				strings.HasPrefix(key, "Span.DestinationService") ||
				// test Samples separately
				strings.HasPrefix(key, "Samples") {
				return true
			}
			return false
		}

		samples := func(val float64) map[string]model.MetricsetSample {
			return map[string]model.MetricsetSample{
				"transaction.duration.count":  {Value: val},
				"transaction.duration.sum.us": {Value: val},
				"transaction.breakdown.count": {Value: val},
				"span.self_time.count":        {Value: val},
				"span.self_time.sum.us":       {Value: val},
			}
		}

		var input metricset
		var out1, out2 model.Metricset
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToMetricsetModel(&input, initializedMetadata(), reqTime, &out1)
		input.Reset()
		// metricset timestamp is always set to request time
		defaultVal.Update(reqTime)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
		assert.Equal(t, samples(defaultVal.Float), out1.Samples)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, initializedMetadata(), reqTime, &out2)
		otherVal.Update(reqTime)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		assert.Equal(t, samples(otherVal.Float), out2.Samples)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
		assert.Equal(t, samples(defaultVal.Float), out1.Samples)
	})
}
