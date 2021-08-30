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

package model

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestTransactionTransform(t *testing.T) {
	id := "123"
	result := "tx result"
	dropped, startedSpans := 5, 14
	name := "mytransaction"
	duration := 65980 * time.Microsecond

	tests := []struct {
		Transaction Transaction
		Output      common.MapStr
		Msg         string
	}{
		{
			Transaction: Transaction{},
			Output: common.MapStr{
				"id":       "",
				"type":     "",
				"duration": common.MapStr{"us": 65980},
				"sampled":  false,
			},
			Msg: "Empty Transaction",
		},
		{
			Transaction: Transaction{
				ID:   id,
				Type: "tx",
			},
			Output: common.MapStr{
				"id":       id,
				"type":     "tx",
				"duration": common.MapStr{"us": 65980},
				"sampled":  false,
			},
			Msg: "SpanCount empty",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Started: &startedSpans},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14},
				"sampled":    false,
			},
			Msg: "SpanCount only contains `started`",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Type:      "tx",
				SpanCount: SpanCount{Dropped: &dropped},
			},
			Output: common.MapStr{
				"id":         id,
				"type":       "tx",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"dropped": 5},
				"sampled":    false,
			},
			Msg: "SpanCount only contains `dropped`",
		},
		{
			Transaction: Transaction{
				ID:        id,
				Name:      name,
				Type:      "tx",
				Result:    result,
				Sampled:   true,
				SpanCount: SpanCount{Started: &startedSpans, Dropped: &dropped},
			},
			Output: common.MapStr{
				"id":         id,
				"name":       "mytransaction",
				"type":       "tx",
				"result":     "tx result",
				"duration":   common.MapStr{"us": 65980},
				"span_count": common.MapStr{"started": 14, "dropped": 5},
				"sampled":    true,
			},
			Msg: "Full Event",
		},
	}

	for idx, test := range tests {
		event := APMEvent{
			Transaction: &test.Transaction,
			Event:       Event{Duration: duration},
		}
		beatEvent := event.BeatEvent(context.Background())
		assert.Equal(t, test.Output, beatEvent.Fields["transaction"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEventsTransformWithMetadata(t *testing.T) {
	hostname := "a.b.c"
	architecture := "darwin"
	platform := "x64"
	id, name, ip, userAgent := "123", "jane", "63.23.123.4", "node-js-2.3"
	url, referer := "https://localhost", "http://localhost"
	serviceName, serviceNodeName, serviceVersion := "myservice", "service-123", "2.1.3"

	request := HTTPRequest{Method: "post", Headers: common.MapStr{}, Referrer: referer}
	response := HTTPResponse{Finished: new(bool), Headers: common.MapStr{"content-type": []string{"text/html"}}}
	txWithContext := APMEvent{
		Service: Service{
			Name:    serviceName,
			Version: serviceVersion,
			Node:    ServiceNode{Name: serviceNodeName},
		},
		Host: Host{
			Name:         name,
			Hostname:     hostname,
			Architecture: architecture,
			OS:           OS{Platform: platform},
		},
		User:      User{ID: id, Name: name},
		UserAgent: UserAgent{Original: userAgent},
		Client:    Client{IP: net.ParseIP(ip)},
		URL:       URL{Original: url},
		Transaction: &Transaction{
			HTTP:    &HTTP{Request: &request, Response: &response},
			Custom:  common.MapStr{"foo.bar": "baz"},
			Message: &Message{QueueName: "routeUser"},
			Sampled: true,
		},
	}

	event := txWithContext.BeatEvent(context.Background())
	assert.Equal(t, common.MapStr{
		"user":       common.MapStr{"id": "123", "name": "jane"},
		"client":     common.MapStr{"ip": ip},
		"source":     common.MapStr{"ip": ip},
		"user_agent": common.MapStr{"original": userAgent},
		"host": common.MapStr{
			"architecture": "darwin",
			"hostname":     "a.b.c",
			"name":         "jane",
			"os": common.MapStr{
				"platform": "x64",
			},
		},
		"service": common.MapStr{
			"name":    serviceName,
			"version": serviceVersion,
			"node":    common.MapStr{"name": serviceNodeName},
		},
		"transaction": common.MapStr{
			"duration": common.MapStr{"us": 0},
			"id":       "",
			"type":     "",
			"sampled":  true,
			"custom": common.MapStr{
				"foo_bar": "baz",
			},
			"message": common.MapStr{"queue": common.MapStr{"name": "routeUser"}},
		},
		"http": common.MapStr{
			"request":  common.MapStr{"method": "post", "referrer": referer},
			"response": common.MapStr{"finished": false, "headers": common.MapStr{"content-type": []string{"text/html"}}},
		},
		"url": common.MapStr{
			"original": url,
		},
	}, event.Fields)
}

func TestTransformTransactionHTTP(t *testing.T) {
	request := HTTPRequest{Method: "post", Body: "<html><marquee>hello world</marquee></html>"}
	event := APMEvent{
		Transaction: &Transaction{
			HTTP: &HTTP{Request: &request},
		},
	}
	beatEvent := event.BeatEvent(context.Background())
	assert.Equal(t, common.MapStr{
		"request": common.MapStr{
			"method":        request.Method,
			"body.original": request.Body,
		},
	}, beatEvent.Fields["http"])
}

func TestTransactionTransformMarks(t *testing.T) {
	tests := []struct {
		Transaction Transaction
		Output      common.MapStr
		Msg         string
	}{
		{
			Transaction: Transaction{
				Marks: TransactionMarks{
					"a.b": TransactionMark{
						"c.d": 123,
					},
				},
			},
			Output: common.MapStr{
				"a_b": common.MapStr{
					"c_d": common.Float(123),
				},
			},
			Msg: "Unsanitized transaction mark names",
		},
	}

	for idx, test := range tests {
		event := APMEvent{Transaction: &test.Transaction}
		beatEvent := event.BeatEvent(context.Background())
		marks, _ := beatEvent.Fields.GetValue("transaction.marks")
		assert.Equal(t, test.Output, marks, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
