package transaction

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	ip := "192.168.0.2"
	timestamp := time.Now()

	app := m.App{Name: "myapp"}
	system := &m.System{
		Hostname:     &hostname,
		Architecture: &architecture,
		Platform:     &platform,
	}

	txValid := Event{Timestamp: timestamp}
	txValidEs := common.MapStr{
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
				"ip":           ip,
			},
			"app": common.MapStr{
				"name":  "myapp",
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
				"ip":           "192.168.0.2",
			},
		},
	}
	spans := []Span{{}}
	txValidWithSpan := Event{Timestamp: timestamp, Spans: spans}
	spanEs := common.MapStr{
		"context": common.MapStr{
			"app": common.MapStr{
				"name":  "myapp",
				"agent": common.MapStr{"name": "", "version": ""},
			},
		},
		"processor": common.MapStr{
			"event": "span",
			"name":  "transaction",
		},
		"span": common.MapStr{
			"duration":    common.MapStr{"us": 0},
			"name":        "",
			"start":       common.MapStr{"us": 0},
			"transaction": common.MapStr{"id": ""},
			"type":        "",
		},
	}

	tests := []struct {
		Payload payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: payload{App: app, Events: []Event{}},
			Output:  nil,
			Msg:     "Payload with empty Event Array",
		},
		{
			Payload: payload{
				App:    app,
				Events: []Event{txValid, txValidWithSpan},
			},
			Output: []common.MapStr{txValidEs, txValidEs, spanEs},
			Msg:    "Payload with multiple Events",
		},
		{
			Payload: payload{
				App:    app,
				System: system,
				Events: []Event{txValid},
			},
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Payload: payload{
				App:    app,
				System: system,
				Events: []Event{txWithContext},
			},
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with App, System and Event with context",
		},
	}

	req, err := http.NewRequest("GET", "_", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = ip
	for idx, test := range tests {
		outputEvents := test.Payload.transform(req)
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp)
		}

	}
}
