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

	"github.com/elastic/apm-server/model/metadata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanEventDecodeFailures(t *testing.T) {
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
	} {
		for _, decodeFct := range []func(interface{}, error) (transform.Transformable, error){V1DecodeEvent, V2DecodeEvent} {
			transformable, err := decodeFct(test.input, test.inpErr)
			assert.Equal(t, test.err, err)
			if test.e != nil {
				event := transformable.(*Event)
				assert.Equal(t, test.e, event)
			} else {
				assert.Nil(t, transformable)
			}
		}
	}
}

func TestDecodeSpanV1(t *testing.T) {
	spanTime, _ := time.Parse(time.RFC3339, "2018-05-30T19:53:17.134Z")
	id, parent, tid := int64(1), int64(12), "abc"
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := map[string]interface{}{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	for idx, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Event
	}{
		{
			//minimal span payload
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z",
			},
			err: nil,
			s: &Event{
				Name:      name,
				Type:      spType,
				Start:     start,
				Duration:  duration,
				Timestamp: spanTime,
			},
		},
		{
			// full valid payload
			input: map[string]interface{}{
				"name": name, "id": 1.0, "type": spType,
				"start": start, "duration": duration,
				"context": context, "parent": 12.0,
				"timestamp":  "2018-05-30T19:53:17.134Z",
				"stacktrace": stacktrace, "transaction_id": tid,
			},
			err: nil,
			s: &Event{
				Id:            &id,
				Name:          name,
				Type:          spType,
				Start:         start,
				Duration:      duration,
				Context:       context,
				Parent:        &parent,
				Timestamp:     spanTime,
				TransactionId: tid,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file", Lineno: 1},
				},
			},
		},
		{
			// ignore distributed tracing data
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z",
				"hex_id":    "hexId", "parent_id": "parentId", "trace_id": "trace_id",
			},
			err: nil,
			s: &Event{
				Name:      name,
				Type:      spType,
				Start:     start,
				Duration:  duration,
				Timestamp: spanTime,
			},
		},
	} {
		span, err := V1DecodeEvent(test.input, test.inpErr)
		assert.Equal(t, test.err, err)
		if test.err != nil {
			assert.Error(t, err)
			assert.Equal(t, test.err, err)
		}
		assert.Equal(t, test.s, span, fmt.Sprintf("Idx <%x>", idx))
	}
}

func TestDecodeSpanV2(t *testing.T) {
	spanTime, _ := time.Parse(time.RFC3339, "2018-05-30T19:53:17.134Z")
	id, parentId, invalidId := "0000000000000000", "FFFFFFFFFFFFFFFF", "invalidId"
	idInt, parentIdInt := int64(-9223372036854775808), int64(9223372036854775807)
	transactionId, traceId := "ABCDEF0123456789", "01234567890123456789abcdefABCDEF"
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := map[string]interface{}{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	fmt.Println(invalidId)
	for idx, test := range []struct {
		input       interface{}
		err, inpErr error
		e           transform.Transformable
	}{
		{
			// invalid id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": invalidId, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: errors.New("strconv.ParseUint: parsing \"invalidId\": invalid syntax"),
			e:   nil,
		},
		{
			// missing traceId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "transaction_id": transactionId,
			},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			// missing transactionId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId,
			},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			// missing id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": "2018-05-30T19:53:17.134Z", "trace_id": traceId, "transaction_id": transactionId,
			},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			// missing parent_id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			// minimal payload
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent_id": parentId,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: nil,
			e: &Event{
				Name:          name,
				Type:          spType,
				Start:         start,
				Duration:      duration,
				Timestamp:     spanTime,
				Id:            &idInt,
				ParentId:      parentId,
				Parent:        &parentIdInt,
				HexId:         id,
				TraceId:       traceId,
				TransactionId: transactionId,
			},
		},
		{
			// full valid payload
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"context": context, "timestamp": "2018-05-30T19:53:17.134Z", "stacktrace": stacktrace,
				"id": id, "parent_id": parentId, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: nil,
			e: &Event{
				Name:      name,
				Type:      spType,
				Start:     start,
				Duration:  duration,
				Context:   context,
				Timestamp: spanTime,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file", Lineno: 1},
				},
				Id:            &idInt,
				HexId:         id,
				TraceId:       traceId,
				ParentId:      parentId,
				Parent:        &parentIdInt,
				TransactionId: transactionId,
			},
		},
	} {
		event, err := V2DecodeEvent(test.input, test.inpErr)
		if test.err != nil {
			if assert.Error(t, err) {
				assert.Equal(t, test.err.Error(), err.Error())
			}
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, test.e, event, fmt.Sprintf("Idx <%x>", idx))
	}
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	parent, tid := int64(12), int64(1)
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
				HexId:      hexId,
				TraceId:    traceId,
				ParentId:   parentId,
				Name:       "myspan",
				Type:       "myspantype",
				Start:      0.65,
				Duration:   1.20,
				Stacktrace: m.Stacktrace{{AbsPath: &path}},
				Context:    common.MapStr{"key": "val"},
				Parent:     &parent,
			},
			Output: common.MapStr{
				"id":       tid,
				"parent":   parent,
				"hex_id":   hexId,
				"duration": common.MapStr{"us": 1200},
				"name":     "myspan",
				"start":    common.MapStr{"us": 650},
				"type":     "myspantype",
				"stacktrace": []common.MapStr{{
					"exclude_from_grouping": false,
					"abs_path":              path,
					"filename":              "",
					"line":                  common.MapStr{"number": 0},
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					}}},
			},
			Msg: "Full Span",
		},
		{
			Event: Event{HexId: hexId, ParentId: parentId},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
				"hex_id":   hexId,
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
	for _, test := range tests {
		output := test.Event.Transform(tctx)
		fields := output[0].Fields["span"]
		assert.Equal(t, test.Output, fields)

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

func TestHexToInt(t *testing.T) {
	testData := []struct {
		input   string
		bitSize int
		valid   bool
		out     int64
	}{
		{"", 16, false, 0},
		{"ffffffffffffffff0", 64, false, 0},                  //value out of range
		{"abcdefx123456789", 64, false, 0},                   //invalid syntax
		{"0123456789abcdef", 64, true, -9141386507638288913}, // 81985529216486895-9223372036854775808
		{"0123456789ABCDEF", 64, true, -9141386507638288913}, // 81985529216486895-9223372036854775808
		{"0000000000000000", 64, true, -9223372036854775808}, // 0-9223372036854775808
		{"ffffffffffffffff", 64, true, 9223372036854775807},  // 18446744073709551615-9223372036854775808
		{"ac03", 16, true, -9223372036854731773},             // 44035-9223372036854775808
		{"acde123456789", 64, true, -9220330920244582519},    //3041116610193289-9223372036854775808
	}
	for _, dt := range testData {
		out, err := hexToInt(dt.input, dt.bitSize)
		if dt.valid {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
		assert.Equal(t, dt.out, out,
			fmt.Sprintf("Expected hexToInt(%v) to return %v", dt.input, dt.out))
	}
}
