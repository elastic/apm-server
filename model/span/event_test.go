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
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanEventDecode(t *testing.T) {
	tid := "longid"
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := map[string]interface{}{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)

	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		e           *Event
	}{
		{input: nil, err: errors.New("Input missing for decoding Event"), e: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		{input: "", err: errors.New("Invalid type for span"), e: nil},
		{
			input: map[string]interface{}{},
			err:   errors.New("Error fetching field"),
			e:     nil,
		},
		{
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"context": context, "stacktrace": stacktrace,
				"transaction_id": tid, "timestamp": timestamp,
			},
			err: nil,
			e: &Event{
				Name:          name,
				Type:          spType,
				Start:         start,
				Duration:      duration,
				Context:       context,
				Timestamp:     timestampParsed,
				TransactionId: &tid,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file", Lineno: 1},
				},
			},
		},
	} {
		for _, decodeFct := range []func(interface{}, error) (transform.Transformable, error){V1DecodeEvent, V2DecodeEvent} {
			transformable, err := decodeFct(test.input, test.inpErr)
			if test.e != nil {
				event := transformable.(*Event)
				assert.Equal(t, test.e, event)
			} else {
				assert.Nil(t, transformable)
			}
			assert.Equal(t, test.err, err)
		}
	}
}

func TestVersionedSpanEvendDecode(t *testing.T) {
	name, spType, start, duration := "foo", "db", 1.2, 3.4
	tid := "longid"
	hexId, parentId := "0147258369012345", "abcdef0123456789"
	traceId := "abcdef0123456789abcdef0123456789"
	id, parent := 1, 12
	timestamp := "2017-05-30T18:53:27.154Z"
	//timestampParsed, _ := time.Parse(time.RFC3339, timestamp)

	// test V1
	input := map[string]interface{}{
		"name": name, "type": spType, "start": start, "duration": duration,
		"id": 1.0, "parent": 12.0, "parent_id": parentId,
		"trace_id":       traceId,
		"transaction_id": tid, timestamp: "timestamp",
	}
	e := &Event{
		Name:          name,
		Type:          spType,
		Start:         start,
		Duration:      duration,
		Id:            &id,
		Parent:        &parent,
		TransactionId: &tid,
	}
	transformable, err := V1DecodeEvent(input, nil)
	assert.NoError(t, err)
	assert.Equal(t, e, transformable.(*Event))

	// test V2
	input = map[string]interface{}{
		"name": name, "type": spType, "start": start, "duration": duration,
		"id": hexId, "parent": 12.0, "parent_id": parentId,
		"trace_id":       traceId,
		"transaction_id": tid, timestamp: "timestamp",
	}
	e = &Event{
		Name:          name,
		Type:          spType,
		Start:         start,
		Duration:      duration,
		HexId:         &hexId,
		ParentId:      &parentId,
		TraceId:       &traceId,
		TransactionId: &tid,
	}
	transformable, err = V2DecodeEvent(input, nil)
	assert.NoError(t, err)
	assert.Equal(t, e, transformable.(*Event))
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	parent := 12
	tid := 1
	service := metadata.Service{Name: "myService"}
	hexId, parentId, traceId := "0147258369012345", "abcdef0123456789", "01234567890123456789abcdefa"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Event: Event{
				Id:         &tid,
				HexId:      &hexId,
				TraceId:    &traceId,
				ParentId:   &parentId,
				Name:       "myspan",
				Type:       "myspantype",
				Start:      0.65,
				Duration:   1.20,
				Stacktrace: m.Stacktrace{{AbsPath: &path}},
				Context:    common.MapStr{"key": "val"},
				Parent:     &parent,
			},
			Output: common.MapStr{
				"duration":  common.MapStr{"us": 1200},
				"id":        1,
				"name":      "myspan",
				"start":     common.MapStr{"us": 650},
				"type":      "myspantype",
				"parent":    12,
				"hex_id":    hexId,
				"trace_id":  traceId,
				"parent_id": parentId,
				"stacktrace": []common.MapStr{{
					"exclude_from_grouping": false,
					"abs_path":              path,
					"filename":              "",
					"line":                  common.MapStr{"number": 0},
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				}},
			},
			Msg: "Full Span",
		},
		{
			Event: Event{HexId: &hexId, ParentId: &parentId},
			Output: common.MapStr{
				"type":      "",
				"start":     common.MapStr{"us": 0},
				"duration":  common.MapStr{"us": 0},
				"name":      "",
				"parent_id": parentId,
				"hex_id":    hexId,
			},
			Msg: "V2 Span without a Stacktrace",
		},
	}

	tctx := &transform.Context{
		Config: transform.Config{SmapMapper: &sourcemap.SmapMapper{}},
		Metadata: metadata.Metadata{
			Service: &service,
		},
	}
	for idx, test := range tests {
		output := test.Event.Transform(tctx)
		fields := output[0].Fields["span"]
		assert.Equal(t, test.Output, fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))

	}
}

func TestEventTransformUseReqTime(t *testing.T) {
	reqTimestamp := "2017-05-30T18:53:27.154Z"
	reqTimestampParsed, err := time.Parse(time.RFC3339, reqTimestamp)
	require.NoError(t, err)

	e := Event{}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)
	assert.Equal(t, reqTimestampParsed, beatEvent[0].Timestamp)
}
