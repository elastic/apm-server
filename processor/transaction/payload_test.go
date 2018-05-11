package transaction

import (
	"errors"
	"testing"

	"github.com/elastic/apm-server/model"

	"github.com/stretchr/testify/assert"

	"time"
)

func TestDecodeContext(t *testing.T) {
	var err error
	ip := "127.0.0.1"
	// pid := 1

	for idx, test := range []struct {
		input    map[string]interface{}
		err      error
		expected *model.TransformContext
	}{
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
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"user":    map[string]interface{}{"ip": ip},
			},
			expected: &model.TransformContext{
				System: &model.System{IP: &ip},
				Service: &model.Service{
					Name: "a",
					Agent: model.Agent{
						Name: "ag", Version: "1.0",
					},
				},
				Process: &model.Process{Pid: 1},
				User:    &model.User{IP: &ip},
			},
		},
	} {
		actual, err := model.DecodeContext(test.input, err)
		if test.err != nil {
			assert.Equal(t, test.err, err, "at index %d", idx)
		} else {
			assert.Equal(t, test.expected, actual, "at index %d", idx)
		}
	}

}

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	// pid := 1
	for idx, test := range []struct {
		input map[string]interface{}
		err   error
		p     []*Transaction
	}{
		{input: nil, err: nil, p: []*Transaction{}},
		{
			input: map[string]interface{}{},
			err:   nil,
			p:     []*Transaction{},
		},
		{
			input: map[string]interface{}{
				"transactions": []interface{}{
					map[string]interface{}{
						"id": "45", "type": "transaction",
						"timestamp": timestamp, "duration": 34.9,
					},
				},
			},
			err: nil,
			p: []*Transaction{
				// Service: m.Service{
				// 	Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				// System:  &m.System{IP: &ip},
				// Process: &m.Process{Pid: pid},
				// User:    &m.User{IP: &ip},
				// Events: []*Transaction{
				&Transaction{
					Id:        "45",
					Type:      "transaction",
					Timestamp: timestampParsed,
					Duration:  34.9,
					Spans:     []*Span{},
				},
				// },
			},
		},
	} {
		Payload, err := DecodePayload(test.input)
		assert.Equal(t, test.p, Payload, "at index %d", idx)
		assert.Equal(t, test.err, err, "at index %d", idx)
	}
}

// TODO: fix this
// func TestPayloadTransform(t *testing.T) {
// 	hostname := "a.b.c"
// 	architecture := "darwin"
// 	platform := "x64"
// 	timestamp := time.Now()

// 	service := m.Service{Name: "myservice"}
// 	system := &m.System{
// 		Hostname:     &hostname,
// 		Architecture: &architecture,
// 		Platform:     &platform,
// 	}

// 	txValid := Transaction{Timestamp: timestamp}
// 	txValidEs := common.MapStr{
// 		"context": common.MapStr{
// 			"service": common.MapStr{
// 				"name":  "myservice",
// 				"agent": common.MapStr{"name": "", "version": ""},
// 			},
// 		},
// 		"processor": common.MapStr{
// 			"event": "transaction",
// 			"name":  "transaction",
// 		},
// 		"transaction": common.MapStr{
// 			"duration": common.MapStr{"us": 0},
// 			"id":       "",
// 			"type":     "",
// 			"sampled":  true,
// 		},
// 	}

// 	txValidWithSystem := common.MapStr{
// 		"processor": common.MapStr{
// 			"event": "transaction",
// 			"name":  "transaction",
// 		},
// 		"transaction": common.MapStr{
// 			"duration": common.MapStr{"us": 0},
// 			"id":       "",
// 			"type":     "",
// 			"sampled":  true,
// 		},
// 		"context": common.MapStr{
// 			"system": common.MapStr{
// 				"hostname":     hostname,
// 				"architecture": architecture,
// 				"platform":     platform,
// 			},
// 			"service": common.MapStr{
// 				"name":  "myservice",
// 				"agent": common.MapStr{"name": "", "version": ""},
// 			},
// 		},
// 	}
// 	txWithContext := Transaction{Timestamp: timestamp, Context: common.MapStr{"foo": "bar", "user": common.MapStr{"id": "55"}}}
// 	txWithContextEs := common.MapStr{
// 		"processor": common.MapStr{
// 			"event": "transaction",
// 			"name":  "transaction",
// 		},
// 		"transaction": common.MapStr{
// 			"duration": common.MapStr{"us": 0},
// 			"id":       "",
// 			"type":     "",
// 			"sampled":  true,
// 		},
// 		"context": common.MapStr{
// 			"foo": "bar", "user": common.MapStr{"id": "55"},
// 			"service": common.MapStr{
// 				"name":  "myservice",
// 				"agent": common.MapStr{"name": "", "version": ""},
// 			},
// 			"system": common.MapStr{
// 				"hostname":     "a.b.c",
// 				"architecture": "darwin",
// 				"platform":     "x64",
// 			},
// 		},
// 	}
// 	spans := []*Span{{}}
// 	txValidWithSpan := Transaction{Timestamp: timestamp, Spans: spans}
// 	spanEs := common.MapStr{
// 		"context": common.MapStr{
// 			"service": common.MapStr{
// 				"name":  "myservice",
// 				"agent": common.MapStr{"name": "", "version": ""},
// 			},
// 		},
// 		"processor": common.MapStr{
// 			"event": "span",
// 			"name":  "transaction",
// 		},
// 		"span": common.MapStr{
// 			"duration": common.MapStr{"us": 0},
// 			"name":     "",
// 			"start":    common.MapStr{"us": 0},
// 			"type":     "",
// 		},
// 		"transaction": common.MapStr{"id": ""},
// 	}

// 	tests := []struct {
// 		Payload Payload
// 		Output  []common.MapStr
// 		Msg     string
// 	}{
// 		{
// 			Payload: Payload{Service: service, Events: []*Transaction{}},
// 			Output:  nil,
// 			Msg:     "Payload with empty Event Array",
// 		},
// 		{
// 			Payload: Payload{
// 				Service: service,
// 				Events:  []*Transaction{&txValid, &txValidWithSpan},
// 			},
// 			Output: []common.MapStr{txValidEs, txValidEs, spanEs},
// 			Msg:    "Payload with multiple Events",
// 		},
// 		{
// 			Payload: Payload{
// 				Service: service,
// 				System:  system,
// 				Events:  []*Transaction{&txValid},
// 			},
// 			Output: []common.MapStr{txValidWithSystem},
// 			Msg:    "Payload with System and Event",
// 		},
// 		{
// 			Payload: Payload{
// 				Service: service,
// 				System:  system,
// 				Events:  []*Transaction{&txWithContext},
// 			},
// 			Output: []common.MapStr{txWithContextEs},
// 			Msg:    "Payload with Service, System and Event with context",
// 		},
// 	}

// 	for idx, test := range tests {
// 		outputEvents := test.Payload.Transform(config.TransformConfig{}, model.TransformContext{})
// 		for j, outputEvent := range outputEvents {
// 			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
// 			assert.Equal(t, timestamp, outputEvent.Timestamp)
// 		}
// 	}
// }
