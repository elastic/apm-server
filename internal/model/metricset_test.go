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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestMetricset(t *testing.T) {
	tests := []struct {
		Metricset *Metricset
		Output    mapstr.M
		Msg       string
	}{
		{
			Metricset: &Metricset{},
			Output:    mapstr.M{},
			Msg:       "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{Name: "raj"},
			Output: mapstr.M{
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
			Output: mapstr.M{
				"a.counter":  612.0,
				"some.gauge": 9.16,
				"_metric_descriptions": mapstr.M{
					"a.counter": mapstr.M(nil),
					"some.gauge": mapstr.M(nil),
				},
			},
			Msg: "Payload with valid metric.",
		},
		{
			Metricset: &Metricset{
				TimeseriesInstanceID: "foo",
				DocCount:             6,
			},
			Output: mapstr.M{
				"timeseries": mapstr.M{"instance": "foo"},
				"_doc_count": int64(6),
			},
			Msg: "Timeseries instance and _doc_count",
		},
		{
			Metricset: &Metricset{
				Samples: map[string]MetricsetSample{
					"latency_histogram": {
						Type: "histogram",
						Unit: "s",
						Histogram: Histogram{
							Counts: []int64{1, 2, 3},
							Values: []float64{1.1, 2.2, 3.3},
						},
					},
					"request_summary": {
						Type: "summary",
						SummaryMetric: SummaryMetric{
							Count: 10,
							Sum:   123.456,
						},
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
			Output: mapstr.M{
				"latency_histogram": mapstr.M{
					"counts": []int64{1, 2, 3},
					"values": []float64{1.1, 2.2, 3.3},
				},
				"request_summary": mapstr.M{
					"sum":         123.456,
					"value_count": int64(10),
				},
				"just_type": 123.0,
				"just_unit": 0.99,
				"_metric_descriptions": mapstr.M{
					"latency_histogram": mapstr.M{
						"type": "histogram",
						"unit": "s",
					},
					"request_summary": mapstr.M{
						"type": "summary",
					},
					"just_type": mapstr.M{
						"type": "counter",
					},
					"just_unit": mapstr.M{
						"unit": "percent",
					},
				},
			},
			Msg: "Payload with metric type and unit.",
		},
	}

	for idx, test := range tests {
		event := APMEvent{Metricset: test.Metricset}
		outputEvent := event.BeatEvent()
		assert.Equal(t, test.Output, outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestTransformMetricsetTransaction(t *testing.T) {
	event := APMEvent{
		Processor: MetricsetProcessor,
		Transaction: &Transaction{
			Name:   "transaction_name",
			Type:   "transaction_type",
			Result: "transaction_result",
			DurationHistogram: Histogram{
				Counts: []int64{1, 2, 3},
				Values: []float64{4.5, 6.0, 9.0},
			},
		},
		Metricset: &Metricset{Name: "transaction"},
	}
	beatEvent := event.BeatEvent()
	assert.Equal(t, mapstr.M{
		"processor":      mapstr.M{"name": "metric", "event": "metric"},
		"metricset.name": "transaction",
		"transaction": mapstr.M{
			"name":   "transaction_name",
			"type":   "transaction_type",
			"result": "transaction_result",
			"duration.histogram": mapstr.M{
				"counts": []int64{1, 2, 3},
				"values": []float64{4.5, 6.0, 9.0},
			},
		},
	}, beatEvent.Fields)
}

func TestTransformMetricsetSpan(t *testing.T) {
	event := APMEvent{
		Processor: MetricsetProcessor,
		Span: &Span{
			Type:    "span_type",
			Subtype: "span_subtype",
			SelfTime: AggregatedDuration{
				Count: 123,
				Sum:   time.Millisecond,
			},
			DestinationService: &DestinationService{
				Resource: "destination_service_resource",
				ResponseTime: AggregatedDuration{
					Count: 456,
					Sum:   time.Second,
				},
			},
		},
		Metricset: &Metricset{Name: "span"},
	}
	beatEvent := event.BeatEvent()
	assert.Equal(t, mapstr.M{
		"processor":      mapstr.M{"name": "metric", "event": "metric"},
		"metricset.name": "span",
		"span": mapstr.M{
			"type":    "span_type",
			"subtype": "span_subtype",
			"self_time": mapstr.M{
				"count":  123,
				"sum.us": int64(1000),
			},
			"destination": mapstr.M{
				"service": mapstr.M{
					"resource": "destination_service_resource",
					"response_time": mapstr.M{
						"count":  456,
						"sum.us": int64(1000000),
					},
				},
			},
		},
	}, beatEvent.Fields)
}
