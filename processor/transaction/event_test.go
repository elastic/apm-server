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
	id, trType, name, result := "123", "type", "foo()", "555"
	duration := 1.67
	context := map[string]interface{}{"a": "b"}
	marks := map[string]interface{}{"k": "b"}
	dropped := 12
	spanCount := map[string]interface{}{
		"dropped": map[string]interface{}{
			"total": dropped,
		},
	}
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	sampled := true

	for _, test := range []struct {
		input interface{}
		err   error
		e     *Event
	}{
		{input: nil, err: nil, e: &Event{}},
		{input: "", err: errors.New("Invalid type for transaction event"), e: &Event{}},
		{
			input: map[string]interface{}{"timestamp": 123},
			err:   errors.New("Invalid type for field"),
			e:     &Event{},
		},
		{
			input: map[string]interface{}{
				"id": id, "type": trType, "name": &name, "result": &result,
				"duration": duration, "timestamp": timestamp,
				"context": context, "marks": marks, "sampled": &sampled,
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
				Spans: []*Span{
					&Span{Name: "span", Type: "db", Start: 1.2, Duration: 2.3},
				},
			},
		},
	} {
		event := &Event{}
		out := event.decode(test.input)
		assert.Equal(t, test.e, event)
		assert.Equal(t, test.err, out)
	}

	var e *Event
	assert.Nil(t, e.decode("a"), nil)
	assert.Nil(t, e)
}

func TestEventTransform(t *testing.T) {

	id := "123"
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
				Spans:     []*Span{},
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
