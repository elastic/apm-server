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

package span

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
)

func TestDecodeSpan(t *testing.T) {
	spanTime := time.Date(2018, 5, 30, 19, 53, 17, 134*1e6, time.UTC)
	timestampEpoch := json.Number(fmt.Sprintf("%d", spanTime.UnixNano()/1000))
	id, parentId := "0000000000000000", "FFFFFFFFFFFFFFFF"
	transactionId, traceId := "ABCDEF0123456789", "01234567890123456789abcdefABCDEF"
	name, spType := "foo", "db"
	start, duration, ageMillis := 1.2, 3.4, 1577958057123
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user, link, rowsAffected := "db01", "select *", "sql", "joe", "other.db.com", 34
	address, port := "localhost", 8080
	destServiceType, destServiceName, destServiceResource := "db", "elasticsearch", "elasticsearch"
	context := map[string]interface{}{
		"a":    "b",
		"tags": map[string]interface{}{"a": "tag", "tag-key": 17},
		"http": map[string]interface{}{"method": "GET", "status_code": json.Number("200"), "url": url},
		"db": map[string]interface{}{
			"instance": instance, "statement": statement, "type": dbType,
			"user": user, "link": link, "rows_affected": json.Number("34")},
		"destination": map[string]interface{}{
			"address": address,
			"port":    float64(port),
			"service": map[string]interface{}{
				"type":     destServiceType,
				"name":     destServiceName,
				"resource": destServiceResource,
			},
		},
		"message": map[string]interface{}{
			"queue": map[string]interface{}{"name": "foo"},
			"age":   map[string]interface{}{"ms": json.Number("1577958057123")}},
	}
	subtype := "postgresql"
	action, action2 := "query", "query.custom"
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "foo",
	}}

	for name, test := range map[string]struct {
		input interface{}
		// we don't use a regular `error.New` here, because some errors are of a different type
		err          string
		experimental bool
		event        *Event
	}{
		"no input":     {input: nil, err: errInvalidType.Error()},
		"invalid type": {input: "", err: errInvalidType.Error()},
		"transaction id wrong type": {
			input: map[string]interface{}{"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "transaction_id": 123},
			err: "expected integer or null, but got string",
		},
		"no trace_id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "transaction_id": transactionId,
			},
			err: "missing properties: \"trace_id\"",
		},
		"no id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: "missing properties: \"id\"",
		},
		"no parent_id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: "missing properties: \"parent_id\"",
		},
		"invalid stacktrace": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "stacktrace": []interface{}{"foo"},
			},
			err: "expected object, but got string",
		},
		"minimal payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId,
			},
			event: &Event{
				Name:      name,
				Type:      "db",
				Subtype:   &subtype,
				Action:    &action2,
				Duration:  duration,
				Timestamp: spanTime,
				ParentId:  parentId,
				Id:        id,
				TraceId:   traceId,
			},
		},
		"event experimental=false": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
				"context": map[string]interface{}{"experimental": 123},
			},
			event: &Event{
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentId:      parentId,
				Id:            id,
				TraceId:       traceId,
				TransactionId: &transactionId,
			},
		},
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
				"context": map[string]interface{}{"foo": 123},
			},
			event: &Event{
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentId:      parentId,
				Id:            id,
				TraceId:       traceId,
				TransactionId: &transactionId,
			},
			experimental: true,
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
				"context": map[string]interface{}{"experimental": 123},
			},
			event: &Event{
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentId:      parentId,
				Id:            id,
				TraceId:       traceId,
				TransactionId: &transactionId,
				Experimental:  123,
			},
			experimental: true,
		},
		"full valid payload": {
			input: map[string]interface{}{
				"name": name, "type": "messaging", "subtype": subtype, "action": action, "start": start,
				"duration": duration, "context": context, "timestamp": timestampEpoch, "stacktrace": stacktrace,
				"id": id, "parent_id": parentId, "trace_id": traceId, "transaction_id": transactionId,
			},
			event: &Event{
				Name:      name,
				Type:      "messaging",
				Subtype:   &subtype,
				Action:    &action,
				Start:     &start,
				Duration:  duration,
				Timestamp: spanTime,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: &name},
				},
				Labels:        common.MapStr{"a": "tag", "tag-key": 17},
				Id:            id,
				TraceId:       traceId,
				ParentId:      parentId,
				TransactionId: &transactionId,
				HTTP:          &HTTP{Method: &method, StatusCode: &statusCode, URL: &url},
				DB: &DB{
					Instance:     &instance,
					Statement:    &statement,
					Type:         &dbType,
					UserName:     &user,
					Link:         &link,
					RowsAffected: &rowsAffected,
				},
				Destination: &Destination{Address: &address, Port: &port},
				DestinationService: &DestinationService{
					Type:     &destServiceType,
					Name:     &destServiceName,
					Resource: &destServiceResource,
				},
				Message: &m.Message{
					QueueName: &name,
					AgeMillis: &ageMillis},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			span, err := Decode(test.input, spanTime, metadata.Metadata{}, test.experimental)
			if test.err == "" {
				require.Nil(t, err)
				assert.Equal(t, test.event, span)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	start := 0.65
	serviceName, serviceVersion, env := "myService", "1.2", "staging"
	service := metadata.Service{Name: &serviceName, Version: &serviceVersion, Environment: &env}
	hexId, parentId, traceId := "0147258369012345", "abcdef0123456789", "01234567890123456789abcdefa"
	subtype := "amqp"
	action := "publish"
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6,
		time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user, rowsAffected := "db01", "select *", "sql", "jane", 5
	metadataLabels := common.MapStr{"label.a": "a", "label.b": "b", "c": 1}
	address, port := "127.0.0.1", 8080
	destServiceType, destServiceName, destServiceResource := "db", "elasticsearch", "elasticsearch"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{Timestamp: timestamp},
			Output: common.MapStr{
				"processor": common.MapStr{"event": "span", "name": "transaction"},
				"service":   common.MapStr{"name": serviceName, "environment": env},
				"span": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"name":     "",
					"type":     "",
				},
				"labels":    metadataLabels,
				"timestamp": common.MapStr{"us": timestampUs},
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Event: Event{
				Id:         hexId,
				TraceId:    traceId,
				ParentId:   parentId,
				Name:       "myspan",
				Type:       "myspantype",
				Subtype:    &subtype,
				Action:     &action,
				Start:      &start,
				Duration:   1.20,
				Stacktrace: m.Stacktrace{{AbsPath: &path}},
				Labels:     common.MapStr{"label.a": 12},
				HTTP:       &HTTP{Method: &method, StatusCode: &statusCode, URL: &url},
				DB: &DB{
					Instance:     &instance,
					Statement:    &statement,
					Type:         &dbType,
					UserName:     &user,
					RowsAffected: &rowsAffected},
				Destination: &Destination{Address: &address, Port: &port},
				DestinationService: &DestinationService{
					Type:     &destServiceType,
					Name:     &destServiceName,
					Resource: &destServiceResource,
				},
				Message: &m.Message{QueueName: &action},
			},
			Output: common.MapStr{
				"span": common.MapStr{
					"id":       hexId,
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
					"message": common.MapStr{"queue": common.MapStr{"name": "publish"}},
				},
				"labels":      common.MapStr{"label.a": 12, "label.b": "b", "c": 1},
				"processor":   common.MapStr{"event": "span", "name": "transaction"},
				"service":     common.MapStr{"name": serviceName, "environment": env},
				"trace":       common.MapStr{"id": traceId},
				"parent":      common.MapStr{"id": parentId},
				"destination": common.MapStr{"address": address, "ip": address, "port": port},
			},
			Msg: "Full Span",
		},
	}

	for _, test := range tests {
		test.Event.Metadata = metadata.Metadata{Service: &service, Labels: metadataLabels}
		output := test.Event.Transform(nil, nil, &sourcemap.Store{})
		fields := output[0].Fields
		assert.Equal(t, test.Output, fields)
	}
}

func TestEventTransformUseReqTimePlusStart(t *testing.T) {
	reqTimestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	start := 1234.8
	event, error := Decode(map[string]interface{}{
		"start":     start,
		"duration":  0.0,
		"type":      "db",
		"id":        "abc",
		"name":      "name",
		"trace_id":  "abd",
		"parent_id": "tx1",
	}, reqTimestampParsed, metadata.Metadata{}, false)
	require.NoError(t, error)

	adjustedParsed := time.Date(2017, 5, 30, 18, 53, 28, 388.8*1e6, time.UTC)
	assert.Equal(t, adjustedParsed, event.Timestamp)
}
