package transaction

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadTransform(t *testing.T) {
	serviceName := "myservice"
	service := m.Service{Name: &serviceName}

	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	timestamp := time.Now()
	id, userId := "111", 55
	truthy := true

	system := &m.System{
		Hostname:     &hostname,
		Architecture: &architecture,
		Platform:     &platform,
	}
	spName, spName2, spType, spType2 := "s1", "s2", "t1", "t2"

	spans := []*Span{
		{Name: &spName, Type: &spType},
		{Name: &spName2, Type: &spType2},
	}
	txValidWithSpan := &Event{Timestamp: timestamp, Id: &id, Spans: spans}
	spanEs1 := common.MapStr{
		"context": common.MapStr{"service": common.MapStr{"name": &serviceName}},
		"processor": common.MapStr{
			"event": "span",
			"name":  "transaction",
		},
		"span":        common.MapStr{"type": &spType, "name": &spName},
		"transaction": common.MapStr{"id": &id},
	}
	spanEs2 := common.MapStr{
		"context": common.MapStr{"service": common.MapStr{"name": &serviceName}},
		"processor": common.MapStr{
			"event": "span",
			"name":  "transaction",
		},
		"span":        common.MapStr{"type": &spType2, "name": &spName2},
		"transaction": common.MapStr{"id": &id},
	}

	txValid := &Event{Timestamp: timestamp, Id: &id}
	txValidEs := common.MapStr{
		"context": common.MapStr{"service": common.MapStr{"name": &serviceName}},
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{"sampled": &truthy, "id": &id},
	}

	txValidWithSystem := common.MapStr{
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{"sampled": &truthy, "id": &id},
		"context": common.MapStr{
			"service": common.MapStr{"name": &serviceName},
			"system": common.MapStr{
				"hostname":     &hostname,
				"architecture": &architecture,
				"platform":     &platform,
			},
		},
	}

	txWithContext := &Event{
		Timestamp: timestamp,
		Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"id": &userId}},
	}
	txWithContextEs := common.MapStr{
		"processor": common.MapStr{
			"event": "transaction",
			"name":  "transaction",
		},
		"transaction": common.MapStr{"sampled": &truthy},
		"context": common.MapStr{
			"foo": "bar", "user": common.MapStr{"id": &userId},
			"service": common.MapStr{"name": &serviceName},
			"system": common.MapStr{
				"hostname":     &hostname,
				"architecture": &architecture,
				"platform":     &platform,
			},
		},
	}

	tests := []struct {
		Payload *payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: &payload{Service: service, Events: []*Event{}},
			Output:  nil,
			Msg:     "Payload with empty Event Array",
		},
		{
			Payload: &payload{
				Service: service,
				Events:  []*Event{txValid, txValidWithSpan},
			},
			Output: []common.MapStr{txValidEs, txValidEs, spanEs1, spanEs2},
			Msg:    "Payload with multiple Events",
		},
		{
			Payload: &payload{
				Service: service,
				System:  system,
				Events:  []*Event{txValid},
			},
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Payload: &payload{
				Service: service,
				System:  system,
				Events:  []*Event{txWithContext},
			},
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with Service, System and Event with context",
		},
	}

	for idx, test := range tests {
		outputEvents := test.Payload.transform(&pr.Config{})
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp)
		}

	}
}
