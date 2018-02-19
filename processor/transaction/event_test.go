package transaction

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestEventTransform(t *testing.T) {
	var ev *Event

	id := "123"
	result := "tx result"
	sampled := false
	dropped := 5
	name := "mytransaction"
	eventType := "request"
	duration := 65.98
	durationMicros := 65980
	truthy := true
	sid1, name1 := 111, "io"

	tests := []struct {
		Event  *Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event:  ev,
			Output: nil,
			Msg:    "Nil Event",
		},
		{
			Event:  &Event{},
			Output: common.MapStr{"sampled": &truthy},
			Msg:    "Empty Event",
		},
		{
			Event: &Event{
				Id:        &id,
				Name:      &name,
				Type:      &eventType,
				Result:    &result,
				Timestamp: time.Now(),
				Duration:  &duration,
				Context:   common.MapStr{"foo": "bar"},
				Spans:     []*Span{&Span{Id: &sid1, Name: &name1}},
				Sampled:   &sampled,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
			},
			Output: common.MapStr{
				"id":         &id,
				"name":       &name,
				"type":       &eventType,
				"result":     &result,
				"duration":   common.MapStr{"us": &durationMicros},
				"span_count": common.MapStr{"dropped": common.MapStr{"total": &dropped}},
				"sampled":    new(bool),
			},
			Msg: "Full Event",
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
