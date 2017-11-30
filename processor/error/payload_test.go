package error

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	svc := m.Service{Name: "myservice"}
	timestamp := time.Now()

	tests := []struct {
		Payload payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: payload{Service: svc, Events: []Event{}},
			Output:  nil,
			Msg:     "Empty Event Array",
		},
		{
			Payload: payload{Service: svc, Events: []Event{{Timestamp: timestamp}}},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myservice",
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
				Service: svc,
				Events: []Event{{
					Timestamp: timestamp,
					Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
					Log:       baseLog(),
					Exception: &Exception{
						Message:    "exception message",
						Stacktrace: m.Stacktrace{m.StacktraceFrame{Filename: "myFile"}},
					},
					Transaction: &struct{ Id string }{Id: "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
				}},
			},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"foo": "bar", "user": common.MapStr{"email": "m@m.com"},
						"service": common.MapStr{
							"name":  "myservice",
							"agent": common.MapStr{"name": "", "version": ""},
						},
					},
					"error": common.MapStr{
						"grouping_key": "1d1e44ffdf01cad5117a72fd42e4fdf4",
						"log":          common.MapStr{"message": "error log message"},
						"exception": common.MapStr{
							"message": "exception message",
							"stacktrace": []common.MapStr{{
								"filename": "myFile",
								"line":     common.MapStr{"number": 0},
								"sourcemap": common.MapStr{
									"error":   "AbsPath, Colno, Service Name and Version mandatory for sourcemapping.",
									"updated": false,
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
		outputEvents := test.Payload.transform(&utility.SourcemapAccessor{})
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
