package error

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	app := m.App{Name: "myapp"}
	timestamp := time.Now()

	tests := []struct {
		Payload payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: payload{App: app, Events: []Event{}},
			Output:  nil,
			Msg:     "Empty Event Array",
		},
		{
			Payload: payload{App: app, Events: []Event{{Timestamp: timestamp}}},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"app": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myapp",
						},
					},
					"error": common.MapStr{
						"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
					},
					"processor": common.MapStr{"event": "error", "name": "error"},
				},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Payload: payload{
				App: app,
				Events: []Event{{
					Timestamp: timestamp,
					Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
					Exception: baseException(),
					Log:       baseLog(),
				}},
			},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"foo": "bar", "user": common.MapStr{"email": "m@m.com"},
						"app": common.MapStr{
							"name":  "myapp",
							"agent": common.MapStr{"name": "", "version": ""},
						},
					},
					"error": common.MapStr{
						"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
						"exception":    common.MapStr{"message": "exception message"},
						"log":          common.MapStr{"message": "error log message"},
					},
					"processor": common.MapStr{"event": "error", "name": "error"},
				},
			},
			Msg: "Payload with Event with Context.",
		},
	}

	for idx, test := range tests {
		outputEvents := test.Payload.transform()
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
