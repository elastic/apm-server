package transaction

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
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
				"transactions": []interface{}{
					map[string]interface{}{
						"id": "45", "type": "transaction",
						"timestamp": timestamp, "duration": 34.9,
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
					&Event{
						Id:        "45",
						Type:      "transaction",
						Timestamp: timestampParsed,
						Duration:  34.9,
						Spans:     []*Span{},
					},
				},
			},
		},
	} {
		Payload, err := DecodePayload(test.input)
		assert.Equal(t, test.p, Payload)
		assert.Equal(t, test.err, err)
	}
}

func TestPayloadTransform(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	timestamp := time.Now()

	service := m.Service{Name: "myservice"}
	system := &m.System{
		Hostname:     &hostname,
		Architecture: &architecture,
		Platform:     &platform,
	}

	txValid := Event{Timestamp: timestamp}
	txValidEs := common.MapStr{
		"context": common.MapStr{
			"service": common.MapStr{
				"name":  "myservice",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
	}

	txValidWithSystem := common.MapStr{
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
		"context": common.MapStr{
			"system": common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
				"platform":     platform,
			},
			"service": common.MapStr{
				"name":  "myservice",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
	}
	txWithContext := Event{Timestamp: timestamp, Context: common.MapStr{"foo": "bar", "user": common.MapStr{"id": "55"}}}
	txWithContextEs := common.MapStr{
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
		},
		"context": common.MapStr{
			"foo": "bar", "user": common.MapStr{"id": "55"},
			"service": common.MapStr{
				"name":  "myservice",
				"agent": common.MapStr{"name": "", "version": ""},
			},
			"system": common.MapStr{
				"hostname":     "a.b.c",
				"architecture": "darwin",
				"platform":     "x64",
			},
		},
	}
	spans := []*Span{{}}
	txValidWithSpan := Event{Timestamp: timestamp, Spans: spans}
	spanEs := common.MapStr{
		"context": common.MapStr{
			"service": common.MapStr{
				"name":  "myservice",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
		"processor": common.MapStr{
			"event": "span",
			"name":  "transaction",
		},
		"span": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"name":     "",
			"start":    common.MapStr{"us": 0},
			"type":     "",
		},
		"transaction": common.MapStr{"id": ""},
	}

	tests := []struct {
		Payload Payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: Payload{Service: service, Events: []*Event{}},
			Output:  nil,
			Msg:     "Payload with empty Event Array",
		},
		{
			Payload: Payload{
				Service: service,
				Events:  []*Event{&txValid, &txValidWithSpan},
			},
			Output: []common.MapStr{txValidEs, txValidEs, spanEs},
			Msg:    "Payload with multiple Events",
		},
		{
			Payload: Payload{
				Service: service,
				System:  system,
				Events:  []*Event{&txValid},
			},
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Payload: Payload{
				Service: service,
				System:  system,
				Events:  []*Event{&txWithContext},
			},
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with Service, System and Event with context",
		},
	}

	for idx, test := range tests {
		outputEvents := test.Payload.Transform(config.Config{})
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp)
		}
	}
}
