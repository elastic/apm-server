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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

func TestDecodeSpan(t *testing.T) {
	spanTime, _ := time.Parse(time.RFC3339, "2018-05-30T19:53:17.134Z")
	timestampEpoch := json.Number(fmt.Sprintf("%d", spanTime.UnixNano()/1000))
	id, parentId := "0000000000000000", "FFFFFFFFFFFFFFFF"
	transactionId, traceId := "ABCDEF0123456789", "01234567890123456789abcdefABCDEF"
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user := "db01", "select *", "sql", "joe"
	context := map[string]interface{}{
		"a":    "b",
		"tags": map[string]interface{}{"a": "tag", "tag.key": 17},
		"http": map[string]interface{}{"method": "GET", "status_code": json.Number("200"), "url": url},
		"db":   map[string]interface{}{"instance": instance, "statement": statement, "type": dbType, "user": user},
	}
	subtype := "postgresql"
	action, action2 := "query", "query.custom"
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	testdata := []struct {
		input interface{}
		// we don't use a regular `error.New` here, because some errors are of a different type
		err    string
		inpErr error
		e      transform.Transformable
	}{
		{input: nil, err: "Input missing for decoding Event"},
		{input: nil, inpErr: errors.New("a"), err: "a"},
		{input: "", err: "Invalid type for span"},
		{
			input: map[string]interface{}{},
			err:   "Error fetching field",
		},
		{
			// transaction id is wrong type
			input: map[string]interface{}{"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "transaction_id": 123},
			err: "Error fetching field",
		},
		{
			// missing traceId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "transaction_id": transactionId,
			},
			err: utility.FetchErr.Error(),
		},
		{
			// missing transactionId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId,
			},
			err: utility.FetchErr.Error(),
		},
		{
			// missing id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: utility.FetchErr.Error(),
		},
		{
			// missing parent_id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: utility.FetchErr.Error(),
		},
		{
			// minimal payload
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			e: &Event{
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
				TransactionId: transactionId,
			},
		},
		{
			// full valid payload
			input: map[string]interface{}{
				"name": name, "type": "external.request", "subtype": subtype, "action": action, "start": start,
				"duration": duration, "context": context, "timestamp": timestampEpoch, "stacktrace": stacktrace,
				"id": id, "parent_id": parentId, "trace_id": traceId, "transaction_id": transactionId,
			},
			e: &Event{
				Name:      name,
				Type:      "external.request",
				Subtype:   &subtype,
				Action:    &action,
				Start:     &start,
				Duration:  duration,
				Context:   context,
				Timestamp: spanTime,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file", Lineno: 1},
				},
				Labels:        common.MapStr{"a": "tag", "tag.key": 17},
				Id:            id,
				TraceId:       traceId,
				ParentId:      parentId,
				TransactionId: transactionId,
				Http:          &http{Method: &method, StatusCode: &statusCode, Url: &url},
				Db:            &db{Instance: &instance, Statement: &statement, Type: &dbType, UserName: &user},
			},
		},
	}
	for idx, test := range testdata {
		span, err := DecodeEvent(test.input, test.inpErr)
		if test.err == "" {
			assert.Equal(t, test.e, span, fmt.Sprintf("Idx <%v>", idx))
		} else {
			assert.EqualError(t, err, test.err, fmt.Sprintf("Idx <%v>", idx))
		}
	}
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	start := 0.65
	serviceName := "myService"
	service := metadata.Service{Name: &serviceName}
	hexId, parentId, traceId := "0147258369012345", "abcdef0123456789", "01234567890123456789abcdefa"
	subtype := "myspansubtype"
	action := "myspanquery"
	timestamp, _ := time.Parse(time.RFC3339, "2019-01-03T15:17:04.908596+01:00")
	timestampUs := timestamp.UnixNano() / 1000
	method, statusCode, url := "get", 200, "http://localhost"
	instance, statement, dbType, user := "db01", "select *", "sql", "jane"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{Timestamp: timestamp},
			Output: common.MapStr{
				"processor": common.MapStr{"event": "span", "name": "transaction"},
				"service":   common.MapStr{"name": "myService"},
				"span": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"name":     "",
					"type":     "",
				},
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
				Context:    common.MapStr{"key": "val"},
				Labels:     common.MapStr{"label.a": 12},
				Http:       &http{Method: &method, StatusCode: &statusCode, Url: &url},
				Db:         &db{Instance: &instance, Statement: &statement, Type: &dbType, UserName: &user},
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
						"filename":              "",
						"line":                  common.MapStr{"number": 0},
						"sourcemap": common.MapStr{
							"error":   "Colno mandatory for sourcemapping.",
							"updated": false,
						}}},
					"db": common.MapStr{
						"instance":  instance,
						"statement": statement,
						"type":      dbType,
						"user":      common.MapStr{"name": user},
					},
					"http": common.MapStr{
						"url":      common.MapStr{"original": url},
						"response": common.MapStr{"status_code": statusCode},
						"method":   "get",
					},
				},
				"labels":    common.MapStr{"label.a": 12},
				"processor": common.MapStr{"event": "span", "name": "transaction"},
				"service":   common.MapStr{"name": "myService"},
				"timestamp": common.MapStr{"us": int64(float64(timestampUs) + start*1000)},
				"trace":     common.MapStr{"id": traceId},
				"parent":    common.MapStr{"id": parentId},
			},
			Msg: "Full Span",
		},
	}

	tctx := &transform.Context{
		Config:      transform.Config{SmapMapper: &sourcemap.SmapMapper{}},
		Metadata:    metadata.Metadata{Service: &service},
		RequestTime: timestamp,
	}
	for _, test := range tests {
		output := test.Event.Transform(tctx)
		fields := output[0].Fields
		assert.Equal(t, test.Output, fields)
	}
}

func TestEventTransformUseReqTimePlusStart(t *testing.T) {
	reqTimestampParsed, err := time.Parse(time.RFC3339, "2017-05-30T18:53:27.154Z")
	require.NoError(t, err)
	start := 1234.8
	e := Event{Start: &start}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)

	adjustedParsed, err := time.Parse(time.RFC3339, "2017-05-30T18:53:28.3888Z")
	assert.Equal(t, adjustedParsed, beatEvent[0].Timestamp)
}
