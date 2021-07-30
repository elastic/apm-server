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
		eventBase := initializedMetadata()
		input := modeldecoder.Input{RequestTime: now, Base: eventBase}
		str := `{"me":{"sa":{"xds":{"v":2048}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedMetricset(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Metricset)
		assert.Equal(t, map[string]model.MetricsetSample{"transaction.duration.sum.us": {Value: 2048}}, batch[0].Metricset.Samples)
		assert.Equal(t, now, batch[0].Metricset.Timestamp)
		modeldecodertest.AssertStructValues(t, &batch[0], metadataExceptions(), modeldecodertest.DefaultValues())

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
	t.Run("metricset-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			if
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
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToMetricsetModel(&input, reqTime, &out1)
		input.Reset()
		// metricset timestamp is always set to request time
		defaultVal.Update(reqTime)
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		assert.Equal(t, samples(defaultVal.Float), out1.Metricset.Samples)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetricsetModel(&input, reqTime, &out2)
		otherVal.Update(reqTime)
		modeldecodertest.AssertStructValues(t, out2.Metricset, exceptions, otherVal)
		assert.Equal(t, samples(otherVal.Float), out2.Metricset.Samples)
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		assert.Equal(t, samples(defaultVal.Float), out1.Metricset.Samples)
	})
}
