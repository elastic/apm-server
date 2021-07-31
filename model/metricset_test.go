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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestMetricset(t *testing.T) {
	timestamp := time.Now()
	resource := "external-service"

	const (
		trType   = "request"
		trName   = "GET /"
		trResult = "HTTP 2xx"

		spType    = "db"
		spSubtype = "sql"

		eventOutcome = "success"
	)

	tests := []struct {
		Metricset *Metricset
		Output    common.MapStr
		Msg       string
	}{
		{
			Metricset: &Metricset{Timestamp: timestamp},
			Output: common.MapStr{
				"processor": common.MapStr{"event": "metric", "name": "metric"},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{Timestamp: timestamp, Name: "raj"},
			Output: common.MapStr{
				"processor":      common.MapStr{"event": "metric", "name": "metric"},
				"metricset.name": "raj",
			},
			Msg: "Payload with metricset name.",
		},
		{
			Metricset: &Metricset{
				Timestamp: timestamp,
				Samples: map[string]MetricsetSample{
					"a.counter":  {Value: 612},
					"some.gauge": {Value: 9.16},
				},
			},
			Output: common.MapStr{
				"processor":  common.MapStr{"event": "metric", "name": "metric"},
				"a.counter":  612.0,
				"some.gauge": 9.16,
			},
			Msg: "Payload with valid metric.",
		},
		{
			Metricset: &Metricset{
				Timestamp:   timestamp,
				Span:        MetricsetSpan{Type: spType, Subtype: spSubtype},
				Transaction: MetricsetTransaction{Type: trType, Name: trName},
				Samples: map[string]MetricsetSample{
					"span.self_time.count": {Value: 123},
				},
			},
			Output: common.MapStr{
				"processor":   common.MapStr{"event": "metric", "name": "metric"},
				"transaction": common.MapStr{"type": trType, "name": trName},
				"span": common.MapStr{
					"type": spType, "subtype": spSubtype,
				},
				"span.self_time.count": 123.0,
			},
			Msg: "Payload with breakdown metrics.",
		},
		{
			Metricset: &Metricset{
				Timestamp: timestamp,
				Event:     MetricsetEventCategorization{Outcome: eventOutcome},
				Transaction: MetricsetTransaction{
					Type:   trType,
					Name:   trName,
					Result: trResult,
					Root:   true,
				},
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
				"processor":  common.MapStr{"event": "metric", "name": "metric"},
				"event":      common.MapStr{"outcome": eventOutcome},
				"timeseries": common.MapStr{"instance": "foo"},
				"transaction": common.MapStr{
					"type":   trType,
					"name":   trName,
					"result": trResult,
					"root":   true,
				},
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
				Timestamp: timestamp,
				Span: MetricsetSpan{Type: spType, Subtype: spSubtype, DestinationService: DestinationService{
					Resource: resource,
				}},
				Samples: map[string]MetricsetSample{
					"destination.service.response_time.count":  {Value: 40},
					"destination.service.response_time.sum.us": {Value: 500000},
				},
			},
			Output: common.MapStr{
				"processor": common.MapStr{"event": "metric", "name": "metric"},
				"span": common.MapStr{
					"type": spType, "subtype": spSubtype,
					"destination": common.MapStr{"service": common.MapStr{"resource": resource}},
				},
				"destination.service.response_time.count":  40.0,
				"destination.service.response_time.sum.us": 500000.0,
			},
			Msg: "Payload with destination service.",
		},
		{
			Metricset: &Metricset{
				Timestamp: timestamp,
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
				"processor": common.MapStr{"event": "metric", "name": "metric"},
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
		event := APMEvent{
			Metricset: test.Metricset,
		}
		outputEvents := event.appendBeatEvent(context.Background(), nil)
		require.Len(t, outputEvents, 1)
		assert.Equal(t, test.Output, outputEvents[0].Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
		assert.Equal(t, timestamp, outputEvents[0].Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
	}
}
