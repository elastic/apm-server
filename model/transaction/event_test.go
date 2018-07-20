// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package transaction

import (
	"errors"
	"fmt"
	"testing"

	"github.com/elastic/apm-server/transform"

	"github.com/stretchr/testify/assert"

	"time"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

func TestTransactionEventDecode(t *testing.T) {
	id, trType, name, result := "123", "type", "foo()", "555"
	duration := 1.67
	context := map[string]interface{}{"a": "b"}
	marks := map[string]interface{}{"k": "b"}
	dropped := 12
	spanCount := map[string]interface{}{
		"dropped": map[string]interface{}{
			"total": 12.0,
		},
	}
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	sampled := true

	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		e           *Event
	}{
		{input: nil, err: nil, e: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		{input: "", err: errors.New("Invalid type for transaction event"), e: nil},
		{
			input: map[string]interface{}{"timestamp": 123},
			err:   errors.New("Error fetching field"),
			e: &Event{
				Id: "", Type: "", Name: nil, Result: nil,
				Duration: 0.0, Timestamp: time.Time{},
				Context: nil, Marks: nil, Sampled: nil,
				SpanCount: SpanCount{Dropped: Dropped{Total: nil}},
				Spans:     []*span.Span{},
			},
		},
		{
			input: map[string]interface{}{
				"id": id, "type": trType, "name": name, "result": result,
				"duration": duration, "timestamp": timestamp,
				"context": context, "marks": marks, "sampled": sampled,
				"span_count": spanCount,
				"spans": []interface{}{
					map[string]interface{}{
						"name": "span", "type": "db", "start": 1.2, "duration": 2.3,
					},
				},
			},
			err: nil,
			e: &Event{
				Id: id, Type: trType, Name: &name, Result: &result,
				Duration: duration, Timestamp: timestampParsed,
				Context: context, Marks: marks, Sampled: &sampled,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
				Spans: []*span.Span{
					&span.Span{Name: "span", Type: "db", Start: 1.2, Duration: 2.3},
				},
			},
		},
	} {
		event, err := DecodeEvent(test.input, test.inpErr)
		assert.Equal(t, test.e, event)
		assert.Equal(t, test.err, err)
	}
}

func TestEventTransform(t *testing.T) {

	id := "123"
	result := "tx result"
	sampled := false
	dropped := 5
	name := "mytransaction"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"id":       "",
				"type":     "",
				"duration": common.MapStr{"us": 0},
				"sampled":  true,
			},
			Msg: "Empty Event",
		},
		{
			Event: Event{
				Id:        id,
				Name:      &name,
				Type:      "tx",
				Result:    &result,
				Timestamp: time.Now(),
				Duration:  65.98,
				Context:   common.MapStr{"foo": "bar"},
				Spans:     []*span.Span{},
				Sampled:   &sampled,
				SpanCount: SpanCount{Dropped: Dropped{Total: &dropped}},
			},
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"dropped": common.MapStr{"total": 5}},
				"sampled":    false,
			},
			Msg: "Full Event",
		},
	}

	tctx := &transform.Context{}

	for idx, test := range tests {
		output := test.Event.Events(tctx)
		assert.Equal(t, test.Output, output[0].Fields["transaction"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestPayloadTransform(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	timestamp := time.Now()

	service := metadata.Service{Name: "myservice"}
	system := &metadata.System{
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
	spans := []*span.Span{{}}
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
		Metadata metadata.Metadata
		Events   []transform.Eventable
		Output   []common.MapStr
		Msg      string
	}{
		{
			Metadata: metadata.Metadata{},
			Events:   []transform.Eventable{},
			Output:   nil,
			Msg:      "Payload with empty Event Array",
		},
		{
			Metadata: metadata.Metadata{
				Service: &service,
			},
			Events: []transform.Eventable{
				&txValid, &txValidWithSpan,
			},
			Output: []common.MapStr{txValidEs, txValidEs, spanEs},
			Msg:    "Payload with multiple Events",
		},
		{
			Metadata: metadata.Metadata{
				Service: &service,
				System:  system,
			},

			Events: []transform.Eventable{&txValid},
			Output: []common.MapStr{txValidWithSystem},
			Msg:    "Payload with System and Event",
		},
		{
			Metadata: metadata.Metadata{
				Service: &service,
				System:  system,
			},
			Events: []transform.Eventable{&txWithContext},
			Output: []common.MapStr{txWithContextEs},
			Msg:    "Payload with Service, System and Event with context",
		},
	}

	for idx, test := range tests {
		var outputEvents []beat.Event

		tctx := &transform.Context{
			Metadata: test.Metadata,
		}
		for _, events := range test.Events {
			outputEvents = append(outputEvents, events.Events(tctx)...)
		}

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v (j: %v); %s", idx, j, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp)
		}
	}
}
