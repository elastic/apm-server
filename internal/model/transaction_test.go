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

func TestTransactionTransformEmpty(t *testing.T) {
	event := APMEvent{Transaction: &Transaction{}}
	beatEvent := event.BeatEvent()
	assert.Empty(t, beatEvent.Fields)
}

func TestTransactionTransform(t *testing.T) {
	id := "123"
	result := "tx result"
	dropped, startedSpans := 5, 14
	name := "mytransaction"
	duration := 65980 * time.Microsecond

	tests := []struct {
		Transaction Transaction
		Output      mapstr.M
		Msg         string
	}{
		{
			Transaction: Transaction{
				ID:   id,
				Type: "tx",
			},
			Output: mapstr.M{
				"id":   id,
				"type": "tx",
			},
			Msg: "SpanCount empty",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Started: &startedSpans},
			},
			Output: mapstr.M{
				"id":         id,
				"type":       "tx",
				"span_count": mapstr.M{"started": 14},
			},
			Msg: "SpanCount only contains `started`",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Dropped: &dropped},
			},
			Output: mapstr.M{
				"id":         id,
				"type":       "tx",
				"span_count": mapstr.M{"dropped": 5},
			},
			Msg: "SpanCount only contains `dropped`",
		},
		{
			Transaction: Transaction{
				ID:                  id,
				Name:                name,
				Type:                "tx",
				Result:              result,
				Sampled:             true,
				SpanCount:           SpanCount{Started: &startedSpans, Dropped: &dropped},
				Root:                true,
				RepresentativeCount: 1.23,
			},
			Output: mapstr.M{
				"id":                   id,
				"name":                 "mytransaction",
				"type":                 "tx",
				"result":               "tx result",
				"span_count":           mapstr.M{"started": 14, "dropped": 5},
				"sampled":              true,
				"root":                 true,
				"representative_count": 1.23,
			},
			Msg: "Full Event",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Name:      name,
				Type:      "tx",
				Result:    result,
				Sampled:   true,
				SpanCount: SpanCount{Started: &startedSpans, Dropped: &dropped},
				DroppedSpansStats: []DroppedSpanStats{
					{
						DestinationServiceResource: "mysql://server:3306",
						Outcome:                    "success",
						Duration: AggregatedDuration{
							Count: 5,
							Sum:   duration,
						},
					},
					{
						DestinationServiceResource: "http://elasticsearch:9200",
						Outcome:                    "unknown",
						Duration: AggregatedDuration{
							Count: 15,
							Sum:   duration,
						},
					},
				},
				Root: true,
			},
			Output: mapstr.M{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"span_count": mapstr.M{"started": 14, "dropped": 5},
				"dropped_spans_stats": []mapstr.M{
					{
						"destination_service_resource": "mysql://server:3306",
						"duration":                     mapstr.M{"count": 5, "sum.us": int64(65980)},
						"outcome":                      "success",
					},
					{
						"destination_service_resource": "http://elasticsearch:9200",
						"duration":                     mapstr.M{"count": 15, "sum.us": int64(65980)},
						"outcome":                      "unknown",
					},
				},
				"sampled": true,
				"root":    true,
			},
			Msg: "Full Event With Dropped Spans Statistics",
		},
	}

	for idx, test := range tests {
		event := APMEvent{
			Processor:   TransactionProcessor,
			Transaction: &test.Transaction,
		}
		beatEvent := event.BeatEvent()
		assert.Equal(t, test.Output, beatEvent.Fields["transaction"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEventsTransformWithMetadata(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	id, name, userAgent := "123", "jane", "node-js-2.3"
	url := "https://localhost"
	serviceName, serviceNodeName, serviceVersion := "myservice", "service-123", "2.1.3"

	txWithContext := APMEvent{
		Processor: TransactionProcessor,
		Service: Service{
			Name:    serviceName,
			Version: serviceVersion,
			Node:    ServiceNode{Name: serviceNodeName},
		},
		Host: Host{
			Name:         name,
			Hostname:     hostname,
			Architecture: architecture,
			OS:           OS{Platform: platform},
		},
		User:      User{ID: id, Name: name},
		UserAgent: UserAgent{Original: userAgent},
		URL:       URL{Original: url},
		Transaction: &Transaction{
			Custom:  mapstr.M{"foo.bar": "baz"},
			Message: &Message{QueueName: "routeUser"},
			Sampled: true,
		},
	}

	event := txWithContext.BeatEvent()
	assert.Equal(t, mapstr.M{
		"processor":  mapstr.M{"name": "transaction", "event": "transaction"},
		"user":       mapstr.M{"id": "123", "name": "jane"},
		"user_agent": mapstr.M{"original": userAgent},
		"host": mapstr.M{
			"architecture": "darwin",
			"hostname":     "a.b.c",
			"name":         "jane",
			"os": mapstr.M{
				"platform": "x64",
			},
		},
		"service": mapstr.M{
			"name":    serviceName,
			"version": serviceVersion,
			"node":    mapstr.M{"name": serviceNodeName},
		},
		"transaction": mapstr.M{
			"sampled": true,
			"custom": mapstr.M{
				"foo_bar": "baz",
			},
			"message": mapstr.M{"queue": mapstr.M{"name": "routeUser"}},
		},
		"url": mapstr.M{
			"original": url,
		},
	}, event.Fields)
}

func TestTransactionTransformMarks(t *testing.T) {
	tests := []struct {
		Transaction Transaction
		Output      mapstr.M
		Msg         string
	}{
		{
			Transaction: Transaction{
				Marks: TransactionMarks{
					"a.b": TransactionMark{
						"c.d": 123,
					},
				},
			},
			Output: mapstr.M{
				"a_b": mapstr.M{
					"c_d": float64(123),
				},
			},
			Msg: "Unsanitized transaction mark names",
		},
	}

	for idx, test := range tests {
		event := APMEvent{Transaction: &test.Transaction}
		beatEvent := event.BeatEvent()
		marks, _ := beatEvent.Fields.GetValue("transaction.marks")
		assert.Equal(t, test.Output, marks, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
