package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"

	app := m.App{Name: "myapp"}
	system := &m.System{
		Hostname:     &hostname,
		Architecture: &architecture,
		Platform:     &platform,
	}

	ts := "2017-05-09T15:04:05.999999Z"

	txValid := Event{Timestamp: ts}
	txValidEs := common.MapStr{
		"@timestamp": ts,
		"context": common.MapStr{
			"app": common.MapStr{
				"name":  "myapp",
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
			"name":     "",
			"type":     "",
		},
	}

	txValidWithSystem := common.MapStr{
		"@timestamp": ts,
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"name":     "",
			"type":     "",
		},
		"context": common.MapStr{
			"system": common.MapStr{
				"hostname":     hostname,
				"architecture": architecture,
				"platform":     platform,
			},
			"app": common.MapStr{
				"name":  "myapp",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
	}
	txWithContext := Event{Timestamp: ts, Context: common.MapStr{"foo": "bar", "user": common.MapStr{"id": "55"}}}
	txWithContextEs := common.MapStr{
		"@timestamp": ts,

		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"name":     "",
			"type":     "",
		},
		"context": common.MapStr{
			"foo": "bar", "user": common.MapStr{"id": "55"},
			"app": common.MapStr{
				"name":  "myapp",
				"agent": common.MapStr{"name": "", "version": ""},
			},
			"system": common.MapStr{
				"hostname":     "a.b.c",
				"architecture": "darwin",
				"platform":     "x64",
			},
		},
	}
	traces := []Trace{{}}
	txValidWithTrace := Event{Timestamp: ts, Traces: traces}
	traceEs := common.MapStr{
		"@timestamp": ts,
		"context": common.MapStr{
			"app": common.MapStr{
				"name":  "myapp",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
		"processor": common.MapStr{
			"event": "trace",
			"name":  "transaction",
		},
		"trace": common.MapStr{
			"duration":       common.MapStr{"us": 0},
			"name":           "",
			"start":          common.MapStr{"us": 0},
			"transaction_id": "",
			"type":           "",
		},
	}

	tests := []struct {
		Payload Payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: Payload{App: app, Events: []Event{}},
			Output:  nil,
			Msg:     "Payload with empty Event Array",
		},
		{
			Payload: Payload{
				App:    app,
				Events: []Event{txValid, txValidWithTrace},
			},
			Output: []common.MapStr{txValidEs, txValidEs, traceEs},
			Msg:    "Payload with multiple Events",
		},
		{
			Payload: Payload{
				App:    app,
				System: system,
				Events: []Event{txValid},
			},
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Payload: Payload{
				App:    app,
				System: system,
				Events: []Event{txWithContext},
			},
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with App, System and Event with context",
		},
	}

	for idx, test := range tests {
		output := test.Payload.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
