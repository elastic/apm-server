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

package modeldecoder

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/tests"
)

func TestDecodeSpan(t *testing.T) {
	requestTime := time.Now()
	spanTime := time.Date(2018, 5, 30, 19, 53, 17, 134*1e6, time.UTC)
	timestampEpoch := json.Number(fmt.Sprintf("%d", spanTime.UnixNano()/1000))
	id, parentID := "0000000000000000", "FFFFFFFFFFFFFFFF"
	transactionID, traceID := "ABCDEF0123456789", "01234567890123456789abcdefABCDEF"
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user, link, rowsAffected := "db01", "select *", "sql", "joe", "other.db.com", 34
	address, port := "localhost", 8080
	destServiceType, destServiceName, destServiceResource := "db", "elasticsearch", "elasticsearch"
	context := map[string]interface{}{
		"a":    "b",
		"tags": map[string]interface{}{"a": "tag", "tag_key": 17},
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
		"filename": "file",
	}}

	metadata := metadata.Metadata{
		Service: metadata.Service{Name: "foo"},
	}

	// baseInput holds the minimal valid input. Test-specific input is added to/removed from this.
	baseInput := common.MapStr{
		"id": id, "type": spType, "name": name, "duration": duration, "trace_id": traceID,
	}

	for name, test := range map[string]struct {
		input map[string]interface{}
		cfg   Config
		e     *span.Event
	}{
		"minimal payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "duration": duration, "parent_id": parentID,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceID,
			},
			e: &span.Event{
				Metadata:  metadata,
				Name:      name,
				Type:      "db",
				Subtype:   &subtype,
				Action:    &action2,
				Duration:  duration,
				Timestamp: spanTime,
				ParentID:  &parentID,
				ID:        id,
				TraceID:   &traceID,
			},
		},
		"no timestamp specified, request time + start used": {
			input: map[string]interface{}{
				"name": name, "type": "db", "duration": duration, "parent_id": parentID, "trace_id": traceID, "id": id,
				"start": start,
			},
			e: &span.Event{
				Metadata:  metadata,
				Name:      name,
				Type:      "db",
				Duration:  duration,
				ParentID:  &parentID,
				ID:        id,
				TraceID:   &traceID,
				Start:     &start,
				Timestamp: requestTime.Add(time.Duration(start * float64(time.Millisecond))),
			},
		},
		"event experimental=false": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentID,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceID, "transaction_id": transactionID,
				"context": map[string]interface{}{"experimental": 123},
			},
			e: &span.Event{
				Metadata:      metadata,
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentID:      &parentID,
				ID:            id,
				TraceID:       &traceID,
				TransactionID: &transactionID,
			},
		},
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentID,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceID, "transaction_id": transactionID,
				"context": map[string]interface{}{"foo": 123},
			},
			e: &span.Event{
				Metadata:      metadata,
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentID:      &parentID,
				ID:            id,
				TraceID:       &traceID,
				TransactionID: &transactionID,
			},
			cfg: Config{Experimental: true},
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentID,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceID, "transaction_id": transactionID,
				"context": map[string]interface{}{"experimental": 123},
			},
			e: &span.Event{
				Metadata:      metadata,
				Name:          name,
				Type:          "db",
				Subtype:       &subtype,
				Action:        &action2,
				Start:         &start,
				Duration:      duration,
				Timestamp:     spanTime,
				ParentID:      &parentID,
				ID:            id,
				TraceID:       &traceID,
				TransactionID: &transactionID,
				Experimental:  123,
			},
			cfg: Config{Experimental: true},
		},
		"full valid payload": {
			input: map[string]interface{}{
				"name": name, "type": "messaging", "subtype": subtype, "action": action, "start": start,
				"duration": duration, "context": context, "timestamp": timestampEpoch, "stacktrace": stacktrace,
				"id": id, "parent_id": parentID, "trace_id": traceID, "transaction_id": transactionID,
			},
			e: &span.Event{
				Metadata:  metadata,
				Name:      name,
				Type:      "messaging",
				Subtype:   &subtype,
				Action:    &action,
				Start:     &start,
				Duration:  duration,
				Timestamp: spanTime,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: tests.StringPtr("file")},
				},
				Labels:        common.MapStr{"a": "tag", "tag_key": 17},
				ID:            id,
				TraceID:       &traceID,
				ParentID:      &parentID,
				TransactionID: &transactionID,
				HTTP:          &span.HTTP{Method: &method, StatusCode: &statusCode, URL: &url},
				DB: &span.DB{
					Instance:     &instance,
					Statement:    &statement,
					Type:         &dbType,
					UserName:     &user,
					Link:         &link,
					RowsAffected: &rowsAffected,
				},
				Destination: &span.Destination{Address: &address, Port: &port},
				DestinationService: &span.DestinationService{
					Type:     &destServiceType,
					Name:     &destServiceName,
					Resource: &destServiceResource,
				},
				Message: &m.Message{
					QueueName: tests.StringPtr("foo"),
					AgeMillis: tests.IntPtr(1577958057123)},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				input[k] = v
			}
			span, err := DecodeSpan(Input{
				Raw:         input,
				RequestTime: requestTime,
				Metadata:    metadata,
				Config:      test.cfg,
			})
			require.NoError(t, err)
			assert.Equal(t, test.e, span)
		})
	}
}

func TestDecodeSpanInvalid(t *testing.T) {
	_, err := DecodeSpan(Input{Raw: nil})
	require.EqualError(t, err, "failed to validate span: error validating JSON: input missing")

	_, err = DecodeSpan(Input{Raw: ""})
	require.EqualError(t, err, "failed to validate span: error validating JSON: invalid input type")

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"type": "type",
		"name": "name",
		"id":   "id", "trace_id": "trace_id", "transaction_id": "transaction_id", "parent_id": "parent_id",
		"start": 0.0, "duration": 123.0,
	}
	_, err = DecodeSpan(Input{Raw: baseInput})
	require.NoError(t, err)

	for name, test := range map[string]struct {
		input map[string]interface{}
	}{
		"transaction id wrong type": {
			input: map[string]interface{}{"transaction_id": 123},
		},
		"no trace_id": {
			input: map[string]interface{}{"trace_id": nil},
		},
		"no id": {
			input: map[string]interface{}{"id": nil},
		},
		"no parent_id": {
			input: map[string]interface{}{"parent_id": nil},
		},
		"invalid stacktrace": {
			input: map[string]interface{}{"stacktrace": []interface{}{"foo"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				if v == nil {
					delete(input, k)
				} else {
					input[k] = v
				}
			}
			_, err := DecodeSpan(Input{Raw: input})
			require.Error(t, err)
			t.Logf("%s", err)
		})
	}
}
