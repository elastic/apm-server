package error

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	pid, ip := 1, "127.0.0.1"
	for _, test := range []struct {
		input map[string]interface{}
		err   error
		p     *Payload
	}{
		{input: nil, err: nil, p: nil},
		{
			input: map[string]interface{}{"service": 123},
			err:   errors.New("Invalid type for service"),
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   errors.New("Invalid type for system"),
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   errors.New("Invalid type for process"),
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   errors.New("Invalid type for user"),
		},
		{
			input: map[string]interface{}{},
			err:   nil,
			p: &Payload{
				Service: m.Service{}, System: nil,
				Process: nil, User: nil, Events: []*Event{},
			},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"user":    map[string]interface{}{"ip": ip},
				"errors": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"exception": map[string]interface{}{
							"message": "Exception Msg",
						},
					},
				},
			},
			err: nil,
			p: &Payload{
				Service: m.Service{
					Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
				User:    &m.User{IP: &ip},
				Events: []*Event{
					&Event{Timestamp: timestampParsed,
						Exception: &Exception{Message: "Exception Msg", Stacktrace: m.Stacktrace{}}},
				},
			},
		},
	} {
		payload, err := DecodePayload(test.input)
		assert.Equal(t, test.p, payload)
		assert.Equal(t, test.err, err)
	}
}

func TestPayloadTransform(t *testing.T) {
	svc := m.Service{Name: "myservice"}
	timestamp := time.Now()

	tests := []struct {
		Payload Payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: Payload{Service: svc, Events: []*Event{}},
			Output:  nil,
			Msg:     "Empty Event Array",
		},
		{
			Payload: Payload{Service: svc, Events: []*Event{{Timestamp: timestamp}}},
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
			Payload: Payload{
				Service: svc,
				Events: []*Event{
					&Event{
						Timestamp: timestamp,
						Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
						Log:       baseLog(),
						Exception: &Exception{
							Message:    "exception message",
							Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: "myFile"}},
						},
						Transaction: &Transaction{Id: "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
					},
				},
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
								"exclude_from_grouping": false,
								"filename":              "myFile",
								"line":                  common.MapStr{"number": 0},
								"sourcemap": common.MapStr{
									"error":   "Colno mandatory for sourcemapping.",
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
		conf := config.Config{SmapMapper: &sourcemap.SmapMapper{}}
		outputEvents := test.Payload.Transform(conf)
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
