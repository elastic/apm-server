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

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/transform"
)

func TestTransform(t *testing.T) {
	timestamp := time.Now()
	metadata := Metadata{
		Service: Service{Name: "myservice"},
	}
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
		Output    []common.MapStr
		Msg       string
	}{
		{
			Metricset: nil,
			Output:    nil,
			Msg:       "Nil metric",
		},
		{
			Metricset: &Metricset{Timestamp: timestamp, Metadata: metadata},
			Output: []common.MapStr{
				{
					"data_stream.type":    "metrics",
					"data_stream.dataset": "apm.myservice",
					"processor":           common.MapStr{"event": "metric", "name": "metric"},
					"service": common.MapStr{
						"name": "myservice",
					},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{
				Metadata:  metadata,
				Labels:    common.MapStr{"a_b": "a.b.value"},
				Timestamp: timestamp,
				Samples: []Sample{
					{
						Name:  "a.counter",
						Value: 612,
					},
					{
						Name:  "some.gauge",
						Value: 9.16,
					},
				},
			},
			Output: []common.MapStr{
				{
					"data_stream.type":    "metrics",
					"data_stream.dataset": "apm.myservice",
					"processor":           common.MapStr{"event": "metric", "name": "metric"},
					"service":             common.MapStr{"name": "myservice"},
					"labels":              common.MapStr{"a_b": "a.b.value"},

					"a":    common.MapStr{"counter": float64(612)},
					"some": common.MapStr{"gauge": float64(9.16)},
				},
			},
			Msg: "Payload with valid metric.",
		},
		{
			Metricset: &Metricset{
				Timestamp:   timestamp,
				Metadata:    metadata,
				Span:        MetricsetSpan{Type: spType, Subtype: spSubtype},
				Transaction: MetricsetTransaction{Type: trType, Name: trName},
				Samples: []Sample{{
					Name:  "span.self_time.count",
					Value: 123,
				}},
			},
			Output: []common.MapStr{
				{
					"data_stream.type":    "metrics",
					"data_stream.dataset": "apm.internal.myservice",
					"processor":           common.MapStr{"event": "metric", "name": "metric"},
					"service":             common.MapStr{"name": "myservice"},
					"transaction":         common.MapStr{"type": trType, "name": trName},
					"span": common.MapStr{
						"type": spType, "subtype": spSubtype,
						"self_time": common.MapStr{
							"count": 123.0,
						},
					},
				},
			},
			Msg: "Payload with breakdown metrics.",
		},
		{
			Metricset: &Metricset{
				Timestamp: timestamp,
				Metadata:  metadata,
				Event:     MetricsetEventCategorization{Outcome: eventOutcome},
				Transaction: MetricsetTransaction{
					Type:   trType,
					Name:   trName,
					Result: trResult,
					Root:   true,
				},
				TimeseriesInstanceID: "foo",
				Samples: []Sample{
					{
						Name:   "transaction.duration.histogram",
						Value:  666, // Value is ignored when Counts/Values are specified
						Counts: []int64{1, 2, 3},
						Values: []float64{4.5, 6.0, 9.0},
					},
				},
			},
			Output: []common.MapStr{
				{
					"data_stream.type":    "metrics",
					"data_stream.dataset": "apm.internal.myservice",
					"processor":           common.MapStr{"event": "metric", "name": "metric"},
					"service":             common.MapStr{"name": "myservice"},
					"event":               common.MapStr{"outcome": eventOutcome},
					"timeseries":          common.MapStr{"instance": "foo"},
					"transaction": common.MapStr{
						"type":   trType,
						"name":   trName,
						"result": trResult,
						"root":   true,
						"duration": common.MapStr{
							"histogram": common.MapStr{
								"counts": []int64{1, 2, 3},
								"values": []float64{4.5, 6.0, 9.0},
							},
						},
					},
				},
			},
			Msg: "Payload with transaction duration.",
		},
		{
			Metricset: &Metricset{
				Timestamp: timestamp,
				Metadata:  metadata,
				Span: MetricsetSpan{Type: spType, Subtype: spSubtype, DestinationService: DestinationService{
					Resource: &resource,
				}},
				Samples: []Sample{
					{
						Name:  "destination.service.response_time.count",
						Value: 40,
					},
					{
						Name:  "destination.service.response_time.sum.us",
						Value: 500000,
					},
				},
			},
			Output: []common.MapStr{
				{
					"data_stream.type":    "metrics",
					"data_stream.dataset": "apm.internal.myservice",
					"processor":           common.MapStr{"event": "metric", "name": "metric"},
					"service":             common.MapStr{"name": "myservice"},
					"span": common.MapStr{"type": spType, "subtype": spSubtype,
						"destination": common.MapStr{"service": common.MapStr{"resource": resource}}},
					"destination": common.MapStr{"service": common.MapStr{"response_time": common.MapStr{
						"count": 40.0,
						"sum":   common.MapStr{"us": 500000.0},
					},
					},
					},
				},
			},
			Msg: "Payload with destination service.",
		},
	}

	for idx, test := range tests {
		outputEvents := test.Metricset.Transform(context.Background(), &transform.Config{DataStreams: true})

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
