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
)

func TestTransactionTransformEmpty(t *testing.T) {
	event := APMEvent{Transaction: &Transaction{}}
	beatEvent := event.BeatEvent(context.Background())
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
		Span        Span
		Output      common.MapStr
		Msg         string
	}{
		{
			Transaction: Transaction{},
			Output: common.MapStr{
				"duration": common.MapStr{"us": 65980},
			},
			Msg: "Empty Transaction",
		},
		{
			Transaction: Transaction{
				ID:   id,
				Type: "tx",
			},
			Span: Span{
				Kind: "CLIENT",
			},
			Output: common.MapStr{
				"id":        id,
				"span.kind": "CLIENT",
				"type":      "tx",
				"duration":  common.MapStr{"us": 65980},
			},
			Msg: "SpanCount empty",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Started: &startedSpans},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14},
			},
			Msg: "SpanCount only contains `started`",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Dropped: &dropped},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"dropped": 5},
			},
			Msg: "SpanCount only contains `dropped`",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Name:      name,
				Type:      "tx",
				Result:    result,
				Sampled:   true,
				SpanCount: SpanCount{Started: &startedSpans, Dropped: &dropped},
				Root:      true,
			},
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14, "dropped": 5},
				"sampled":    true,
				"root":       true,
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
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14, "dropped": 5},
				"dropped_spans_stats": []common.MapStr{
					{
						"destination_service_resource": "mysql://server:3306",
						"duration":                     common.MapStr{"count": 5, "sum.us": int64(65980)},
						"outcome":                      "success",
					},
					{
						"destination_service_resource": "http://elasticsearch:9200",
						"duration":                     common.MapStr{"count": 15, "sum.us": int64(65980)},
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
			Span:        &test.Span,
			Event:       Event{Duration: duration},
		}
		beatEvent := event.BeatEvent(context.Background())
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
			Custom:  common.MapStr{"foo.bar": "baz"},
			Message: &Message{QueueName: "routeUser"},
			Sampled: true,
		},
	}

	event := txWithContext.BeatEvent(context.Background())
	assert.Equal(t, common.MapStr{
		"processor":  common.MapStr{"name": "transaction", "event": "transaction"},
		"user":       common.MapStr{"id": "123", "name": "jane"},
		"user_agent": common.MapStr{"original": userAgent},
		"host": common.MapStr{
			"architecture": "darwin",
			"hostname":     "a.b.c",
			"name":         "jane",
			"os": common.MapStr{
				"platform": "x64",
			},
		},
		"service": common.MapStr{
			"name":    serviceName,
			"version": serviceVersion,
			"node":    common.MapStr{"name": serviceNodeName},
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"sampled":  true,
			"custom": common.MapStr{
				"foo_bar": "baz",
			},
			"message": common.MapStr{"queue": common.MapStr{"name": "routeUser"}},
		},
		"url": common.MapStr{
			"original": url,
		},
	}, event.Fields)
}

func TestTransactionTransformMarks(t *testing.T) {
	tests := []struct {
		Transaction Transaction
		Output      common.MapStr
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
			Output: common.MapStr{
				"a_b": common.MapStr{
					"c_d": common.Float(123),
				},
			},
			Msg: "Unsanitized transaction mark names",
		},
	}

	for idx, test := range tests {
		event := APMEvent{Transaction: &test.Transaction}
		beatEvent := event.BeatEvent(context.Background())
		marks, _ := beatEvent.Fields.GetValue("transaction.marks")
		assert.Equal(t, test.Output, marks, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
