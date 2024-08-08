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
		now := modelpb.FromTime(time.Now())
		defaultVal := modeldecodertest.DefaultValues()
		_, eventBase := initializedInputMetadata(defaultVal)
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"metricset":{"timestamp":1599996822281000,"samples":{"a.b":{"value":2048}}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch modelpb.Batch
		require.NoError(t, DecodeNestedMetricset(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Metricset)
		assert.Equal(t, modelpb.FromTime(time.Unix(1599996822, 281000000).UTC()), batch[0].Timestamp)
		assert.Empty(t, cmp.Diff(&modelpb.Metricset{
			Name: "app",
			Samples: []*modelpb.MetricsetSample{{
				Name:  "a.b",
				Value: 2048,
			}},
		}, batch[0].Metricset, protocmp.Transform()))

		// use base event's Timestamp if unspecified in input
		str = `{"metricset":{"samples":{"a.b":{"value":2048}}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = batch[:0]
		require.NoError(t, DecodeNestedMetricset(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Metricset)
		assert.Equal(t, now, batch[0].Timestamp)

		// invalid type
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
		err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToMetricsetModel(t *testing.T) {
	exceptions := func(key string) bool {
		if key == "doc_count" ||
			key == "name" ||
			key == "TimeseriesInstanceID" ||
			key == "interval" ||
			// test Samples separately
			strings.HasPrefix(key, "samples") {
			return true
		}
		return false
	}

	t.Run("faas", func(t *testing.T) {
		var input metricset
		var out modelpb.APMEvent
		input.FAAS.ID.Set("faasID")
		input.FAAS.Coldstart.Set(true)
		input.FAAS.Execution.Set("execution")
		input.FAAS.Trigger.Type.Set("http")
		input.FAAS.Trigger.RequestID.Set("abc123")
		input.FAAS.Name.Set("faasName")
		input.FAAS.Version.Set("1.0.0")
		mapToMetricsetModel(&input, &out)
		assert.Equal(t, "faasID", out.Faas.Id)
		assert.True(t, *out.Faas.ColdStart)
		assert.Equal(t, "execution", out.Faas.Execution)
		assert.Equal(t, "http", out.Faas.TriggerType)
		assert.Equal(t, "abc123", out.Faas.TriggerRequestId)
		assert.Equal(t, "faasName", out.Faas.Name)
		assert.Equal(t, "1.0.0", out.Faas.Version)
	})

	t.Run("metricset-values", func(t *testing.T) {
		var input metricset
		var out1, out2 modelpb.APMEvent
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Transaction.Reset() // tested by TestDecodeMetricsetInternal

		mapToMetricsetModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		defaultSamples := []*modelpb.MetricsetSample{
			{
				Name:  defaultVal.Str + "0",
				Type:  modelpb.MetricType(modelpb.MetricType_value[defaultVal.Str]),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: &modelpb.Histogram{
					Counts: repeatUint64(uint64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
			{
				Name:  defaultVal.Str + "1",
				Type:  modelpb.MetricType(modelpb.MetricType_value[defaultVal.Str]),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: &modelpb.Histogram{
					Counts: repeatUint64(uint64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
			{
				Name:  defaultVal.Str + "2",
				Type:  modelpb.MetricType(modelpb.MetricType_value[defaultVal.Str]),
				Unit:  defaultVal.Str,
				Value: defaultVal.Float,
				Histogram: &modelpb.Histogram{
					Counts: repeatUint64(uint64(defaultVal.Int), defaultVal.N),
					Values: repeatFloat64(defaultVal.Float, defaultVal.N),
				},
			},
		}
		assert.Empty(t, cmp.Diff(defaultSamples, out1.Metricset.Samples,
			cmpopts.SortSlices(func(x, y *modelpb.MetricsetSample) bool {
				return x.Name < y.Name
			}),
			protocmp.Transform(),
		))

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		input.Transaction.Reset()
		mapToMetricsetModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Metricset, exceptions, otherVal)
		otherSamples := []*modelpb.MetricsetSample{
			{
				Name:  otherVal.Str + "0",
				Type:  modelpb.MetricType(modelpb.MetricType_value[otherVal.Str]),
				Unit:  otherVal.Str,
				Value: otherVal.Float,
				Histogram: &modelpb.Histogram{
					Counts: repeatUint64(uint64(otherVal.Int), otherVal.N),
					Values: repeatFloat64(otherVal.Float, otherVal.N),
				},
			},
			{
				Name:  otherVal.Str + "1",
				Type:  modelpb.MetricType(modelpb.MetricType_value[otherVal.Str]),
				Unit:  otherVal.Str,
				Value: otherVal.Float,
				Histogram: &modelpb.Histogram{
					Counts: repeatUint64(uint64(otherVal.Int), otherVal.N),
					Values: repeatFloat64(otherVal.Float, otherVal.N),
				},
			},
		}
		assert.Empty(t, cmp.Diff(otherSamples, out2.Metricset.Samples,
			cmpopts.SortSlices(func(x, y *modelpb.MetricsetSample) bool {
				return x.Name < y.Name
			}),
			protocmp.Transform(),
		))
		modeldecodertest.AssertStructValues(t, out1.Metricset, exceptions, defaultVal)
		assert.Empty(t, cmp.Diff(defaultSamples, out1.Metricset.Samples,
			cmpopts.SortSlices(func(x, y *modelpb.MetricsetSample) bool {
				return x.Name < y.Name
			}),
			protocmp.Transform(),
		))
	})
}

func TestDecodeMetricsetInternal(t *testing.T) {
	var batch modelpb.Batch

	// There are no known metrics in the samples. Because "transaction" is set,
	// the metricset will be omitted.
	err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"transaction.duration.count": {"value": 456},
				"transaction.duration.sum.us": {"value": 789}
			},
			"transaction": {
				"name": "transaction_name",
				"type": "transaction_type"
			}
		}
	}`)), &modeldecoder.Input{Base: &modelpb.APMEvent{}}, &batch)
	require.NoError(t, err)
	require.Empty(t, batch)

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
	}`)), &modeldecoder.Input{Base: &modelpb.APMEvent{}}, &batch)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(modelpb.Batch{{
		Metricset: &modelpb.Metricset{
			Name: "span_breakdown",
		},
		Transaction: &modelpb.Transaction{
			Name: "transaction_name",
			Type: "transaction_type",
		},
		Span: &modelpb.Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: &modelpb.AggregatedDuration{
				Count: 123,
				Sum:   uint64(456 * time.Microsecond),
			},
		},
	}}, batch, protocmp.Transform()))
}

func TestDecodeMetricsetServiceName(t *testing.T) {
	var input = &modeldecoder.Input{
		Base: &modelpb.APMEvent{
			Service: &modelpb.Service{
				Name:        "service_name",
				Version:     "service_version",
				Environment: "service_environment",
			},
		},
	}
	var batch modelpb.Batch

	err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"span.self_time.count": {"value": 123},
				"span.self_time.sum.us": {"value": 456}
			},
			"service": {
				"name": "ow_service_name"
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
	}`)), input, &batch)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(modelpb.Batch{{
		Metricset: &modelpb.Metricset{
			Name: "span_breakdown",
		},
		Service: &modelpb.Service{
			Name:        "ow_service_name",
			Environment: "service_environment",
		},
		Transaction: &modelpb.Transaction{
			Name: "transaction_name",
			Type: "transaction_type",
		},
		Span: &modelpb.Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: &modelpb.AggregatedDuration{
				Count: 123,
				Sum:   uint64(456 * time.Microsecond),
			},
		},
	}}, batch, protocmp.Transform()))
}

func TestDecodeMetricsetServiceNameAndVersion(t *testing.T) {
	var input = &modeldecoder.Input{
		Base: &modelpb.APMEvent{
			Service: &modelpb.Service{
				Name:        "service_name",
				Version:     "service_version",
				Environment: "service_environment",
			},
		},
	}
	var batch modelpb.Batch

	err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"span.self_time.count": {"value": 123},
				"span.self_time.sum.us": {"value": 456}
			},
			"service": {
				"name": "ow_service_name",
				"version": "ow_service_version"
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
	}`)), input, &batch)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(modelpb.Batch{{
		Metricset: &modelpb.Metricset{
			Name: "span_breakdown",
		},
		Service: &modelpb.Service{
			Name:        "ow_service_name",
			Version:     "ow_service_version",
			Environment: "service_environment",
		},
		Transaction: &modelpb.Transaction{
			Name: "transaction_name",
			Type: "transaction_type",
		},
		Span: &modelpb.Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: &modelpb.AggregatedDuration{
				Count: 123,
				Sum:   uint64(456 * time.Microsecond),
			},
		},
	}}, batch, protocmp.Transform()))
}

func TestDecodeMetricsetServiceVersion(t *testing.T) {
	var input = &modeldecoder.Input{
		Base: &modelpb.APMEvent{
			Service: &modelpb.Service{
				Name:        "service_name",
				Version:     "service_version",
				Environment: "service_environment",
			},
		},
	}
	var batch modelpb.Batch

	err := DecodeNestedMetricset(decoder.NewJSONDecoder(strings.NewReader(`{
		"metricset": {
			"timestamp": 0,
			"samples": {
				"span.self_time.count": {"value": 123},
				"span.self_time.sum.us": {"value": 456}
			},
			"service": {
				"version": "ow_service_version"
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
	}`)), input, &batch)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(modelpb.Batch{{
		Metricset: &modelpb.Metricset{
			Name: "span_breakdown",
		},
		Service: &modelpb.Service{
			Name:        "service_name",
			Version:     "service_version",
			Environment: "service_environment",
		},
		Transaction: &modelpb.Transaction{
			Name: "transaction_name",
			Type: "transaction_type",
		},
		Span: &modelpb.Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: &modelpb.AggregatedDuration{
				Count: 123,
				Sum:   uint64(456 * time.Microsecond),
			},
		},
	}}, batch, protocmp.Transform()))
}

func repeatUint64(v uint64, n int) []uint64 {
	vs := make([]uint64, n)
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
