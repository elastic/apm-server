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

	"github.com/elastic/apm-server/utility"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
)

func TestDecodeSpan(t *testing.T) {
	spanTime := time.Date(2018, 5, 30, 19, 53, 17, 134*1e6, time.UTC)
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
		"filename": "file",
	}}

	for name, test := range map[string]struct {
		input interface{}
		cfg   m.Config
		// we don't use a regular `error.New` here, because some errors are of a different type
		err    string
		inpErr error
		e      transform.Transformable
	}{
		"no input":     {input: nil, err: errMissingInput.Error()},
		"input error":  {input: nil, inpErr: errors.New("a"), err: "a"},
		"invalid type": {input: "", err: errInvalidType.Error()},
		"missing required field": {
			input: map[string]interface{}{},
			err:   utility.ErrFetch.Error(),
		},
		"transaction id wrong type": {
			input: map[string]interface{}{"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "transaction_id": 123},
			err: utility.ErrFetch.Error(),
		},
		"no trace_id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "transaction_id": transactionId,
			},
			err: utility.ErrFetch.Error(),
		},
		"no id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: utility.ErrFetch.Error(),
		},
		"no parent_id": {
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: utility.ErrFetch.Error(),
		},
		"invalid stacktrace": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "stacktrace": []interface{}{"foo"},
			},
			err: m.ErrInvalidStacktraceFrameType.Error(),
		},
		"minimal payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId,
			},
			e: &Event{
				Name:      name,
				Type:      "db",
				Subtype:   &subtype,
				Action:    &action2,
				Start:     &start,
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
				TransactionId: &transactionId,
			},
		},
		"event experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
				"context": map[string]interface{}{"foo": 123},
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
				TransactionId: &transactionId,
			},
			cfg: m.Config{Experimental: true},
		},
		"event experimental=true": {
			input: map[string]interface{}{
				"name": name, "type": "db.postgresql.query.custom", "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": timestampEpoch, "id": id, "trace_id": traceId, "transaction_id": transactionId,
				"context": map[string]interface{}{"experimental": 123},
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
				TransactionId: &transactionId,
				Experimental:  123,
			},
			cfg: m.Config{Experimental: true},
		},
		"full valid payload": {
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
				Timestamp: spanTime,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file"},
				},
				Labels:        common.MapStr{"a": "tag", "tag.key": 17},
				Id:            id,
				TraceId:       traceId,
				ParentId:      parentId,
				TransactionId: &transactionId,
				Http:          &http{Method: &method, StatusCode: &statusCode, Url: &url},
				Db:            &db{Instance: &instance, Statement: &statement, Type: &dbType, UserName: &user},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			span, err := DecodeEvent(test.input, test.cfg, test.inpErr)
			if test.err == "" {
				assert.Equal(t, test.e, span)
				return
			}
			if assert.Error(t, err) {
				assert.Equal(t, test.err, err.Error())
			}
		})
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
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6,
		time.FixedZone("+0100", 3600))
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
	reqTimestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	start := 1234.8
	e := Event{Start: &start}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)

	adjustedParsed := time.Date(2017, 5, 30, 18, 53, 28, 388.8*1e6, time.UTC)
	assert.Equal(t, adjustedParsed, beatEvent[0].Timestamp)
}
