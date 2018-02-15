package error

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	sName := "myservice"
	svc := m.Service{Name: &sName}
	timestamp := time.Now()
	groupingKey := "21b2c27fbe338d800826e5ad67db7d0e"
	groupingKeyIdx1 := "d41d8cd98f00b204e9800998ecf8427e"
	msg := "message"
	filename := "my file"
	smapErr := "Colno and Lineno mandatory for sourcemapping."

	tests := []struct {
		Payload payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: payload{Service: svc, Events: []*Event{}},
			Output:  nil,
			Msg:     "Empty Event Array",
		},
		{
			Payload: payload{Service: svc, Events: []*Event{&Event{Timestamp: timestamp}}},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{"name": &sName},
					},
					"error": common.MapStr{
						"grouping_key": &groupingKeyIdx1,
					},
					"processor": common.MapStr{"event": "error", "name": "error"},
				},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Payload: payload{
				Service: svc,
				Events: []*Event{&Event{
					Timestamp: timestamp,
					Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
					Log:       &Log{Message: &msg},
					Exception: &Exception{
						Message:    &msg,
						Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: &filename}},
					},
					Transaction: &struct{ Id string }{Id: "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
				}},
			},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"foo": "bar", "user": common.MapStr{"email": "m@m.com"},
						"service": common.MapStr{"name": &sName},
					},
					"error": common.MapStr{
						"grouping_key": &groupingKey,
						"log":          common.MapStr{"message": &msg},
						"exception": common.MapStr{
							"message": &msg,
							"stacktrace": []common.MapStr{{
								"exclude_from_grouping": new(bool),
								"filename":              &filename,
								"sourcemap": common.MapStr{
									"error":   &smapErr,
									"updated": new(bool),
								},
							}},
						},
					},
					"processor":   common.MapStr{"event": "error", "name": "error"},
					"transaction": common.MapStr{"id": "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
				},
			},
			Msg: "Payload with Event with Context.",
		},
	}

	for idx, test := range tests {
		outputEvents := test.Payload.transform(&pr.Config{SmapMapper: &sourcemap.SmapMapper{}})
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
