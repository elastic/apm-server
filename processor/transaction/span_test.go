package transaction

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestDecodeSpan(t *testing.T) {
	spanTime, _ := time.Parse(time.RFC3339, "2018-05-30T19:53:17.134Z")
	id, parent := int64(1), int64(12)
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := map[string]interface{}{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	for idx, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Span
	}{
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: nil, err: errors.New("Invalid type for span"), s: nil},
		{input: "", err: errors.New("Invalid type for span"), s: nil},
		{
			input: map[string]interface{}{"timestamp": "2018-05-30T19:53:17.134Z"},
			err:   errors.New("Error fetching field"),
			s:     nil,
		},
		{
			//minimal span payload
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z",
			},
			err: nil,
			s: &Span{
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
				"stacktrace": stacktrace,
			},
			err: nil,
			s: &Span{
				Id:        &id,
				Name:      name,
				Type:      spType,
				Start:     start,
				Duration:  duration,
				Context:   context,
				Parent:    &parent,
				Timestamp: spanTime,
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
			s: &Span{
				Name:      name,
				Type:      spType,
				Start:     start,
				Duration:  duration,
				Timestamp: spanTime,
			},
		},
	} {
		span, err := DecodeSpan(test.input, test.inpErr)
		assert.Equal(t, test.err, err)
		if test.err != nil {
			assert.Error(t, err)
			assert.Equal(t, test.err, err)
		}
		assert.Equal(t, test.s, span, fmt.Sprintf("Idx <%x>", idx))
	}
}

func TestDecodeDistributedTracingSpan(t *testing.T) {
	spanTime, _ := time.Parse(time.RFC3339, "2018-05-30T19:53:17.134Z")
	id, parentId, invalidId := "0000000000000000", "FFFFFFFFFFFFFFFF", "invalidId"
	idInt, parentIdInt := int64(-9223372036854775808), int64(9223372036854775807)
	transactionId, traceId := "ABCDEF0123456789", "01234567890123456789abcdefABCDEF"
	idTransactionId := "01234567890123456789abcdefABCDEF-ABCDEF0123456789"
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
		s           *Span
	}{
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: nil, err: errors.New("Invalid type for span"), s: nil},
		{input: "", err: errors.New("Invalid type for span"), s: nil},
		{
			// invalid id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": invalidId, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: errors.New(`strconv.ParseUint: parsing "invalidId": invalid syntax`),
			s:   nil,
		},
		{
			// missing traceId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "transaction_id": transactionId,
			},
			err: errors.New("Error fetching field"),
			s:   nil,
		},
		{
			// missing transactionId
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId,
			},
			err: errors.New("Error fetching field"),
			s:   nil,
		},
		{
			// missing id
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "trace_id": traceId, "transaction_id": transactionId,
			},
			err: errors.New("Error fetching field"),
			s:   nil,
		},
		{
			// minimal payload
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration,
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: nil,
			s: &Span{
				Name:          name,
				Type:          spType,
				Start:         start,
				Duration:      duration,
				Timestamp:     spanTime,
				Id:            &idInt,
				HexId:         &id,
				TraceId:       &traceId,
				TransactionId: &idTransactionId,
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
			s: &Span{
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
				HexId:         &id,
				TraceId:       &traceId,
				ParentId:      &parentId,
				Parent:        &parentIdInt,
				TransactionId: &idTransactionId,
			},
		},
		{
			// ignore single service tracing data
			input: map[string]interface{}{
				"name": name, "type": spType, "start": start, "duration": duration, "parent": int64(12),
				"timestamp": "2018-05-30T19:53:17.134Z", "id": id, "trace_id": traceId, "transaction_id": transactionId,
			},
			err: nil,
			s: &Span{
				Name:          name,
				Type:          spType,
				Start:         start,
				Duration:      duration,
				Timestamp:     spanTime,
				Id:            &idInt,
				HexId:         &id,
				TraceId:       &traceId,
				TransactionId: &idTransactionId,
			},
		},
	} {
		span, err := DecodeDtSpan(test.input, test.inpErr)
		if test.err != nil {
			if assert.Error(t, err) {
				assert.Equal(t, test.err.Error(), err.Error())
			}
		}
		assert.Equal(t, test.s, span, fmt.Sprintf("Idx <%x>", idx))
	}
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
		out, err := HexToInt(dt.input, dt.bitSize)
		if dt.valid {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
		assert.Equal(t, dt.out, out,
			fmt.Sprintf("Expected HexToInt(%v) to return %v", dt.input, dt.out))
	}
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	tid, parent := int64(1), int64(12)
	service := m.Service{Name: "myService"}

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span: Span{},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Span: Span{
				Id:         &tid,
				Name:       "myspan",
				Type:       "myspantype",
				Start:      0.65,
				Duration:   1.20,
				Stacktrace: m.Stacktrace{{AbsPath: &path}},
				Context:    common.MapStr{"key": "val"},
				Parent:     &parent,
			},
			Output: common.MapStr{
				"duration": common.MapStr{"us": 1200},
				"id":       tid,
				"name":     "myspan",
				"start":    common.MapStr{"us": 650},
				"type":     "myspantype",
				"parent":   parent,
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
	}

	for idx, test := range tests {
		output := test.Span.Transform(config.Config{SmapMapper: &sourcemap.SmapMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
