package error

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	app := m.App{Name: "myapp"}
	ts := "2017-05-09T15:04:05.999999Z"
	msg := ""

	tests := []struct {
		Payload Payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: Payload{App: app, Events: []Event{}},
			Output:  nil,
			Msg:     "Empty Event Array",
		},
		{
			Payload: Payload{App: app, Events: []Event{{Timestamp: ts}}},
			Output: []common.MapStr{
				{
					"@timestamp": ts,
					"context": common.MapStr{
						"app": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myapp",
						},
					},
					"error": common.MapStr{
						"checksum": "d41d8cd98f00b204e9800998ecf8427e",
					},
					"processor": common.MapStr{"event": "error", "name": "error"},
				},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Payload: Payload{
				App: app,
				Events: []Event{{
					Timestamp: ts,
					Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
					Exception: Exception{Message: &msg},
					Log:       Log{Message: &msg},
				}},
			},
			Output: []common.MapStr{
				{
					"@timestamp": ts,

					"context": common.MapStr{
						"foo": "bar", "user": common.MapStr{"email": "m@m.com"},
						"app": common.MapStr{
							"name":  "myapp",
							"agent": common.MapStr{"name": "", "version": ""},
						},
					},
					"error": common.MapStr{
						"checksum":  "d41d8cd98f00b204e9800998ecf8427e",
						"exception": common.MapStr{"message": ""},
						"log":       common.MapStr{"message": ""},
					},
					"processor": common.MapStr{"event": "error", "name": "error"},
				},
			},
			Msg: "Payload with Event with Context.",
		},
	}

	for idx, test := range tests {
		output := test.Payload.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
