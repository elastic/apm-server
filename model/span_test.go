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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
)

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	start := 0.65
	serviceName, serviceVersion, env := "myService", "1.2", "staging"
	service := Service{Name: serviceName, Version: serviceVersion, Environment: env}
	hexID, parentID, traceID := "0147258369012345", "abcdef0123456789", "01234567890123456789abcdefa"
	subtype := "amqp"
	action := "publish"
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6,
		time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user, rowsAffected := "db01", "select *", "sql", "jane", 5
	metadataLabels := common.MapStr{"label.a": "a", "label.b": "b", "c": 1}
	metadata := Metadata{Service: service, Labels: metadataLabels}
	address, port := "127.0.0.1", 8080
	destServiceType, destServiceName, destServiceResource := "db", "elasticsearch", "elasticsearch"

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Msg:  "Span without a Stacktrace",
			Span: Span{Timestamp: timestamp, Metadata: metadata},
			Output: common.MapStr{
				"data_stream.type":    "traces",
				"data_stream.dataset": "apm.myservice",
				"processor":           common.MapStr{"event": "span", "name": "transaction"},
				"service":             common.MapStr{"name": serviceName, "environment": env, "version": serviceVersion},
				"span": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"name":     "",
					"type":     "",
				},
				"event":     common.MapStr{"outcome": ""},
				"labels":    common.MapStr{"label_a": "a", "label_b": "b", "c": 1},
				"timestamp": common.MapStr{"us": timestampUs},
			},
		},
		{
			Msg:  "Span with outcome",
			Span: Span{Timestamp: timestamp, Metadata: metadata, Outcome: "success"},
			Output: common.MapStr{
				"data_stream.type":    "traces",
				"data_stream.dataset": "apm.myservice",
				"processor":           common.MapStr{"event": "span", "name": "transaction"},
				"service":             common.MapStr{"name": serviceName, "environment": env, "version": serviceVersion},
				"span": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"name":     "",
					"type":     "",
				},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label_a": "a", "label_b": "b", "c": 1},
				"event":     common.MapStr{"outcome": "success"},
			},
		},
		{
			Msg: "Full Span",
			Span: Span{
				Metadata:            metadata,
				ID:                  hexID,
				TraceID:             traceID,
				ParentID:            parentID,
				Name:                "myspan",
				Type:                "myspantype",
				Subtype:             subtype,
				Action:              action,
				Timestamp:           timestamp,
				Start:               &start,
				Outcome:             "unknown",
				RepresentativeCount: 5,
				Duration:            1.20,
				RUM:                 true,
				Stacktrace:          Stacktrace{{AbsPath: path}},
				Labels:              common.MapStr{"label_a": 12},
				HTTP:                &HTTP{Method: method, StatusCode: statusCode, URL: url},
				DB: &DB{
					Instance:     instance,
					Statement:    statement,
					Type:         dbType,
					UserName:     user,
					RowsAffected: &rowsAffected},
				Destination: &Destination{Address: address, Port: port},
				DestinationService: &DestinationService{
					Type:     destServiceType,
					Name:     destServiceName,
					Resource: destServiceResource,
				},
				Message: &Message{QueueName: "users"},
			},
			Output: common.MapStr{
				"data_stream.type":    "traces",
				"data_stream.dataset": "apm.myservice",
				"span": common.MapStr{
					"id":       hexID,
					"duration": common.MapStr{"us": 1200},
					"name":     "myspan",
					"start":    common.MapStr{"us": 650},
					"type":     "myspantype",
					"subtype":  subtype,
					"action":   action,
					"stacktrace": []common.MapStr{{
						"exclude_from_grouping": false,
						"abs_path":              path,
						"sourcemap": common.MapStr{
							"error":   "Colno mandatory for sourcemapping.",
							"updated": false,
						}}},
					"db": common.MapStr{
						"instance":      instance,
						"statement":     statement,
						"type":          dbType,
						"user":          common.MapStr{"name": user},
						"rows_affected": rowsAffected,
					},
					"http": common.MapStr{
						"url":      common.MapStr{"original": url},
						"response": common.MapStr{"status_code": statusCode},
						"method":   "get",
					},
					"destination": common.MapStr{
						"service": common.MapStr{
							"type":     destServiceType,
							"name":     destServiceName,
							"resource": destServiceResource,
						},
					},
					"message": common.MapStr{"queue": common.MapStr{"name": "users"}},
				},
				"labels":      common.MapStr{"label_a": 12, "label_b": "b", "c": 1},
				"processor":   common.MapStr{"event": "span", "name": "transaction"},
				"service":     common.MapStr{"name": serviceName, "environment": env, "version": serviceVersion},
				"timestamp":   common.MapStr{"us": timestampUs},
				"trace":       common.MapStr{"id": traceID},
				"parent":      common.MapStr{"id": parentID},
				"destination": common.MapStr{"address": address, "ip": address, "port": port},
				"event":       common.MapStr{"outcome": "unknown"},
				"http": common.MapStr{
					"response":       common.MapStr{"status_code": statusCode},
					"request.method": "get",
				},
				"url.original": url,
			},
		},
	}

	for _, test := range tests {
		output := test.Span.appendBeatEvents(context.Background(), &transform.Config{
			DataStreams: true,
			RUM:         transform.RUMConfig{SourcemapStore: &sourcemap.Store{}},
		}, nil)
		fields := output[0].Fields
		assert.Equal(t, test.Output, fields, test.Msg)
	}
}
