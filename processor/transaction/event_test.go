package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/beats/libbeat/common"
)

//TODO: use test enhancer as argument and test error case
func TestEventTransform(t *testing.T) {

	id := "123"
	result := "tx result"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"id":       "",
				"name":     "",
				"type":     "",
				"duration": common.MapStr{"us": 0},
			},
			Msg: "Empty Event",
		},
		{
			Event: Event{
				Id:        id,
				Name:      "mytransaction",
				Type:      "tx",
				Result:    &result,
				Timestamp: time.Now(),
				Duration:  65.98,
				Context:   common.MapStr{"foo": "bar"},
				Traces:    []Trace{},
			},
			Output: common.MapStr{
				"id":       id,
				"name":     "mytransaction",
				"type":     "tx",
				"result":   "tx result",
				"duration": common.MapStr{"us": 65980},
			},
			Msg: "Full Event",
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
