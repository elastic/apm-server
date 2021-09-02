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

package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestMetricset(t *testing.T) {
	tests := []struct {
		Metricset *Metricset
		Output    common.MapStr
		Msg       string
	}{
		{
			Metricset: &Metricset{},
			Output:    common.MapStr{},
			Msg:       "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{Name: "raj"},
			Output: common.MapStr{
				"metricset.name": "raj",
			},
			Msg: "Payload with metricset name.",
		},
		{
			Metricset: &Metricset{
				Samples: map[string]MetricsetSample{
					"a.counter":  {Value: 612},
					"some.gauge": {Value: 9.16},
				},
			},
			Output: common.MapStr{
				"a.counter":  612.0,
				"some.gauge": 9.16,
			},
			Msg: "Payload with valid metric.",
		},
		{
			Metricset: &Metricset{
				Samples: map[string]MetricsetSample{
					"span.self_time.count": {Value: 123},
				},
			},
			Output: common.MapStr{
				"span.self_time.count": 123.0,
			},
			Msg: "Payload with breakdown metrics.",
		},
		{
			Metricset: &Metricset{
				TimeseriesInstanceID: "foo",
				Samples: map[string]MetricsetSample{
					"transaction.duration.histogram": {
						Type:   "histogram",
						Value:  666, // ignored for histogram type
						Counts: []int64{1, 2, 3},
						Values: []float64{4.5, 6.0, 9.0},
					},
				},
				DocCount: 6,
			},
			Output: common.MapStr{
				"timeseries": common.MapStr{"instance": "foo"},
				"transaction.duration.histogram": common.MapStr{
					"counts": []int64{1, 2, 3},
					"values": []float64{4.5, 6.0, 9.0},
				},
				"_metric_descriptions": common.MapStr{
					"transaction.duration.histogram": common.MapStr{
						"type": "histogram",
					},
				},
				"_doc_count": int64(6),
			},
			Msg: "Payload with transaction duration.",
		},
		{
			Metricset: &Metricset{
				Samples: map[string]MetricsetSample{
					"destination.service.response_time.count":  {Value: 40},
					"destination.service.response_time.sum.us": {Value: 500000},
				},
			},
			Output: common.MapStr{
				"destination.service.response_time.count":  40.0,
				"destination.service.response_time.sum.us": 500000.0,
			},
			Msg: "Payload with destination service.",
		},
		{
			Metricset: &Metricset{
				Samples: map[string]MetricsetSample{
					"latency_histogram": {
						Type:   "histogram",
						Unit:   "s",
						Counts: []int64{1, 2, 3},
						Values: []float64{1.1, 2.2, 3.3},
					},
					"just_type": {
						Type:  "counter",
						Value: 123,
					},
					"just_unit": {
						Unit:  "percent",
						Value: 0.99,
					},
				},
			},
			Output: common.MapStr{
				"latency_histogram": common.MapStr{
					"counts": []int64{1, 2, 3},
					"values": []float64{1.1, 2.2, 3.3},
				},
				"just_type": 123.0,
				"just_unit": 0.99,
				"_metric_descriptions": common.MapStr{
					"latency_histogram": common.MapStr{
						"type": "histogram",
						"unit": "s",
					},
					"just_type": common.MapStr{
						"type": "counter",
					},
					"just_unit": common.MapStr{
						"unit": "percent",
					},
				},
			},
			Msg: "Payload with metric type and unit.",
		},
	}

	for idx, test := range tests {
		event := APMEvent{Metricset: test.Metricset}
		outputEvent := event.BeatEvent(context.Background())
		assert.Equal(t, test.Output, outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TransformTransactionMetricset(t *testing.T) {
	event := APMEvent{
		Processor: MetricsetProcessor,
		Transaction: &Transaction{
			Name:   "transaction_name",
			Type:   "transaction_type",
			Result: "transaction_result",
		},
		Metricset: &Metricset{
			Name:                 "transaction",
			TimeseriesInstanceID: "foo",
			DocCount:             6,
			Samples: map[string]MetricsetSample{
				"transaction.duration.histogram": {
					Type:   "histogram",
					Value:  666, // ignored for histogram type
					Counts: []int64{1, 2, 3},
					Values: []float64{4.5, 6.0, 9.0},
				},
			},
		},
	}
	beatEvent := event.BeatEvent(context.Background())
	assert.Equal(t, common.MapStr{
		"metricset": common.MapStr{
			"name": "transaction",
		},
		"timeseries": common.MapStr{"instance": "foo"},
		"transaction.duration.histogram": common.MapStr{
			"counts": []int64{1, 2, 3},
			"values": []float64{4.5, 6.0, 9.0},
		},
		"_metric_descriptions": common.MapStr{
			"transaction.duration.histogram": common.MapStr{
				"type": "histogram",
			},
		},
		"_doc_count": int64(6),
	}, beatEvent.Fields)
}
