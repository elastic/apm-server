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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestSpanTransformEmpty(t *testing.T) {
	var event APMEvent
	event.Span = &Span{}
	beatEvent := event.BeatEvent()
	assert.Empty(t, beatEvent.Fields)
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	hexID := "0147258369012345"
	subtype := "amqp"
	action := "publish"
	duration := time.Millisecond * 1500
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6, time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	instance, statement, dbType, user, rowsAffected := "db01", "select *", "sql", "jane", 5
	destServiceType, destServiceName, destServiceResource := "db", "elasticsearch", "elasticsearch"
	links := []SpanLink{{Span: Span{ID: "linked_span"}, Trace: Trace{ID: "linked_trace"}}}

	tests := []struct {
		Span   Span
		Output mapstr.M
		Msg    string
	}{
		{
			Msg: "Full Span",
			Span: Span{
				ID:                  hexID,
				Name:                "myspan",
				Type:                "myspantype",
				Kind:                "CLIENT",
				Subtype:             subtype,
				Action:              action,
				RepresentativeCount: 5,
				Stacktrace:          Stacktrace{{AbsPath: path}},
				DB: &DB{
					Instance:     instance,
					Statement:    statement,
					Type:         dbType,
					UserName:     user,
					RowsAffected: &rowsAffected,
				},
				DestinationService: &DestinationService{
					Type:     destServiceType,
					Name:     destServiceName,
					Resource: destServiceResource,
				},
				Message:   &Message{QueueName: "users"},
				Composite: &Composite{Count: 10, Sum: 1.1, CompressionStrategy: "exact_match"},
				Links:     links,
			},
			Output: mapstr.M{
				"processor": mapstr.M{"name": "transaction", "event": "span"},
				"event":     mapstr.M{"duration": duration.Nanoseconds()},
				"span": mapstr.M{
					"id":      hexID,
					"name":    "myspan",
					"kind":    "CLIENT",
					"type":    "myspantype",
					"subtype": subtype,
					"action":  action,
					"stacktrace": []mapstr.M{{
						"exclude_from_grouping": false,
						"abs_path":              path,
					}},
					"db": mapstr.M{
						"instance":      instance,
						"statement":     statement,
						"type":          dbType,
						"user":          mapstr.M{"name": user},
						"rows_affected": rowsAffected,
					},
					"destination": mapstr.M{
						"service": mapstr.M{
							"type":     destServiceType,
							"name":     destServiceName,
							"resource": destServiceResource,
						},
					},
					"message": mapstr.M{"queue": mapstr.M{"name": "users"}},
					"composite": mapstr.M{
						"count":                10,
						"sum":                  mapstr.M{"us": 1100},
						"compression_strategy": "exact_match",
					},
					"links": []mapstr.M{{
						"span":  mapstr.M{"id": "linked_span"},
						"trace": mapstr.M{"id": "linked_trace"},
					}},
				},
				"timestamp": mapstr.M{"us": int(timestampUs)},
			},
		},
	}

	for _, test := range tests {
		event := APMEvent{
			Processor: SpanProcessor,
			Span:      &test.Span,
			Timestamp: timestamp,
			Event:     Event{Duration: duration},
		}
		output := event.BeatEvent()
		assert.Equal(t, test.Output, output.Fields, test.Msg)
	}
}

func TestSpanHTTPFields(t *testing.T) {
	event := APMEvent{
		Processor: SpanProcessor,
		Span:      &Span{},
		HTTP: HTTP{
			Version: "2.0",
			Request: &HTTPRequest{
				Method: "get",
			},
			Response: &HTTPResponse{
				StatusCode: 200,
			},
		},
		URL: URL{Original: "http://localhost"},
	}

	output := event.BeatEvent()
	assert.Equal(t, mapstr.M{
		"processor": mapstr.M{
			"name":  "transaction",
			"event": "span",
		},
		"http": mapstr.M{
			"version": event.HTTP.Version,
			"request": mapstr.M{
				"method": event.HTTP.Request.Method,
			},
			"response": mapstr.M{
				"status_code": event.HTTP.Response.StatusCode,
			},
		},
		"url": mapstr.M{
			"original": event.URL.Original,
		},
	}, output.Fields)
}
