package transaction

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/beats/libbeat/common"
)

func TestTransactionEventDecode(t *testing.T) {
	trType, name, result := "type", "foo()", "555"
	id := "01234567-6789-0123-abCD-ABCDEFabcdef"
	dtId, parentId, hexId := "abcDEF0123456789", "0123456789abcdef", "0123456789ABCDEF"
	traceId := "0123456789abcdef0123456789ABCDEF"
	duration := 1.67
	context := map[string]interface{}{"a": "b"}
	marks := map[string]interface{}{"k": "b"}
	dropped := 12
	spanCount := map[string]interface{}{
		"dropped": map[string]interface{}{
			"total": 12.0,
		},
	}
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	sampled := true

	for idx, test := range []struct {
		input       interface{}
		err, inpErr error
		e           *Event
		spans       []*Span
	}{
		{input: nil, err: nil, e: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		{input: "", err: errors.New("Invalid type for transaction event"), e: nil},
		{
			input: map[string]interface{}{"timestamp": timestamp},
			err:   errors.New("Error fetching field"),
			e:     nil,
		},
		{
			// single service tracing format
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "result": result,
				"duration": duration, "timestamp": timestamp,
				"context": context, "marks": marks, "sampled": sampled,
				"span_count": spanCount,
				"spans": []interface{}{
					map[string]interface{}{
						"name": "span", "type": "db", "start": 1.2, "duration": 2.3,
					},
				},
			},
			err: nil,
			e: &Event{
				Id: id, Type: trType, Name: &name, Result: &result,
				Duration: duration, Timestamp: timestampParsed,
				Context: context, Marks: marks, Sampled: &sampled,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
			},
			spans: []*Span{
				&Span{Name: "span", Type: "db", Start: 1.2, Duration: 2.3, Timestamp: timestampParsed, TransactionId: &id},
			},
		},
		{
			// distributed tracing format
			input: map[string]interface{}{
				"id": dtId, "type": trType, "name": name, "result": result,
				"duration": duration, "timestamp": timestamp,
				"context": context, "marks": marks, "sampled": sampled,
				"trace_id": traceId, "parent_id": parentId, "hex_id": hexId,
				"span_count": spanCount,
				"spans": []interface{}{
					map[string]interface{}{
						"name": "span", "type": "db", "start": 1.2, "duration": 2.3,
					},
				},
			},
			err: nil,
			e: &Event{
				Id: "0123456789abcdef0123456789ABCDEF-abcDEF0123456789", Type: trType, Name: &name, Result: &result,
				Duration: duration, Timestamp: timestampParsed, Context: context, Marks: marks, Sampled: &sampled,
				TraceId: &traceId, ParentId: &parentId, HexId: &dtId,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
			},
			spans: nil,
		},
		{
			// distributed tracing format: no traceId
			input: map[string]interface{}{"id": dtId, "duration": duration, "type": trType, "timestamp": timestamp},
			err:   errors.New("Error fetching field"),
			e:     nil,
			spans: nil,
		},
		{
			// distributed tracing format: no parentId
			input: map[string]interface{}{"id": dtId, "duration": duration, "type": trType, "trace_id": traceId, "timestamp": timestamp},
			err:   nil,
			e: &Event{
				Id: "0123456789abcdef0123456789ABCDEF-abcDEF0123456789", Type: trType, Duration: duration,
				Timestamp: timestampParsed, TraceId: &traceId, HexId: &dtId},
			spans: nil,
		},
	} {
		event, spans, err := DecodeEvent(test.input, test.inpErr)
		if test.err != nil {
			assert.Error(t, err)
			assert.Equal(t, test.err, err)
		}
		assert.Equal(t, test.e, event, fmt.Sprintf("Idx <%x>", idx))
		assert.Equal(t, test.spans, spans, fmt.Sprintf("Idx <%x>", idx))
	}
}

func TestEventTransform(t *testing.T) {

	id := "123xad"
	result := "tx result"
	sampled := false
	dropped := 5
	name := "mytransaction"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"id":       "",
				"type":     "",
				"duration": common.MapStr{"us": 0},
				"sampled":  true,
			},
			Msg: "Empty Event",
		},
		{
			Event: Event{
				Id:        id,
				Name:      &name,
				Type:      "tx",
				Result:    &result,
				Timestamp: time.Now(),
				Duration:  65.98,
				Context:   common.MapStr{"foo": "bar"},
				Sampled:   &sampled,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
			},
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"dropped": common.MapStr{"total": 5}},
				"sampled":    false,
			},
			Msg: "Full Event",
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
