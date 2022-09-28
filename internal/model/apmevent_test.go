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
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestAPMEventFields(t *testing.T) {
	pid := 1234
	host := "host"
	hostname := "hostname"
	containerID := "container-123"
	serviceName, serviceNodeName := "myservice", "serviceABC"
	uid := "12321"
	mail := "user@email.com"
	agentName := "elastic-node"
	outcome := "success"
	destinationAddress := "1.2.3.4"
	destinationPort := 1234
	traceID := "trace_id"
	parentID := "parent_id"
	childID := []string{"child_1", "child_2"}
	httpRequestMethod := "post"
	httpRequestBody := "<html><marquee>hello world</marquee></html>"
	coldstart := true
	eventDuration := time.Microsecond

	for _, test := range []struct {
		input  APMEvent
		output mapstr.M
	}{{
		input: APMEvent{
			Agent: Agent{
				Name:    agentName,
				Version: agentVersion,
			},
			Observer:  Observer{Type: "apm-server"},
			Container: Container{ID: containerID},
			Service: Service{
				Name: serviceName,
				Node: ServiceNode{Name: serviceNodeName},
				Origin: &ServiceOrigin{
					ID:      "abc123",
					Name:    serviceName,
					Version: "1.0",
				},
			},
			Host: Host{
				Hostname: hostname,
				Name:     host,
			},
			Client: Client{Domain: "client.domain"},
			Source: Source{
				IP:   netip.MustParseAddr("127.0.0.1"),
				Port: 1234,
				NAT: &NAT{
					IP: netip.MustParseAddr("10.10.10.10"),
				},
			},
			Destination: Destination{Address: destinationAddress, Port: destinationPort},
			Process:     Process{Pid: pid},
			User:        User{ID: uid, Email: mail},
			Event:       Event{Outcome: outcome, Duration: eventDuration},
			Session:     Session{ID: "session_id"},
			URL:         URL{Original: "url"},
			Labels: map[string]LabelValue{
				"a": {Value: "b"},
				"c": {Value: "true"},
				"d": {Values: []string{"true", "false"}},
			},
			NumericLabels: map[string]NumericLabelValue{
				"e": {Value: float64(1234)},
				"f": {Values: []float64{1234, 12311}},
			},
			Message:     "bottle",
			Transaction: &Transaction{},
			Timestamp:   time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6, time.FixedZone("+0100", 3600)),
			Processor:   Processor{Name: "processor_name", Event: "processor_event"},
			Trace:       Trace{ID: traceID},
			Parent:      Parent{ID: parentID},
			Child:       Child{ID: childID},
			HTTP: HTTP{
				Request: &HTTPRequest{
					Method: httpRequestMethod,
					Body:   httpRequestBody,
				},
			},
			FAAS: FAAS{
				ID:               "faasID",
				Coldstart:        &coldstart,
				Execution:        "execution",
				TriggerType:      "http",
				TriggerRequestID: "abc123",
				Name:             "faasName",
				Version:          "1.0.0",
			},
			Cloud: Cloud{
				Origin: &CloudOrigin{
					AccountID:   "accountID",
					Provider:    "aws",
					Region:      "us-west-1",
					ServiceName: "serviceName",
				},
			},
		},
		output: mapstr.M{
			// common fields
			"agent":     mapstr.M{"version": "1.0.0", "name": "elastic-node"},
			"observer":  mapstr.M{"type": "apm-server"},
			"container": mapstr.M{"id": containerID},
			"host":      mapstr.M{"hostname": hostname, "name": host},
			"process":   mapstr.M{"pid": pid},
			"service": mapstr.M{
				"name": "myservice",
				"node": mapstr.M{"name": serviceNodeName},
				"origin": mapstr.M{
					"id":      "abc123",
					"name":    "myservice",
					"version": "1.0",
				},
			},
			"user":   mapstr.M{"id": "12321", "email": "user@email.com"},
			"client": mapstr.M{"domain": "client.domain"},
			"source": mapstr.M{
				"ip":   "127.0.0.1",
				"port": 1234,
				"nat":  mapstr.M{"ip": "10.10.10.10"},
			},
			"destination": mapstr.M{
				"address": destinationAddress,
				"ip":      destinationAddress,
				"port":    destinationPort,
			},
			"event":   mapstr.M{"outcome": outcome, "duration": eventDuration.Nanoseconds()},
			"session": mapstr.M{"id": "session_id"},
			"url":     mapstr.M{"original": "url"},
			"labels": mapstr.M{
				"a": "b",
				"c": "true",
				"d": []string{"true", "false"},
			},
			"numeric_labels": mapstr.M{
				"e": float64(1234),
				"f": []float64{1234, 12311},
			},
			"message": "bottle",
			"trace": mapstr.M{
				"id": traceID,
			},
			"processor": mapstr.M{
				"name":  "processor_name",
				"event": "processor_event",
			},
			"parent": mapstr.M{
				"id": parentID,
			},
			"child": mapstr.M{
				"id": childID,
			},
			"http": mapstr.M{
				"request": mapstr.M{
					"method": "post",
					"body": mapStr{
						"original": httpRequestBody,
					},
				},
			},
			"faas": mapstr.M{
				"id":                 "faasID",
				"coldstart":          true,
				"execution":          "execution",
				"trigger.type":       "http",
				"trigger.request_id": "abc123",
				"name":               "faasName",
				"version":            "1.0.0",
			},
			"cloud": mapstr.M{
				"origin": mapstr.M{
					"account.id":   "accountID",
					"provider":     "aws",
					"region":       "us-west-1",
					"service.name": "serviceName",
				},
			},
		},
	}, {
		input: APMEvent{
			Processor: TransactionProcessor,
			Timestamp: time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6, time.FixedZone("+0100", 3600)),
		},
		output: mapstr.M{
			"processor": mapstr.M{"name": "transaction", "event": "transaction"},
			// timestamp.us is added for transactions, spans, and errors.
			"timestamp": mapstr.M{"us": 1546525024908596},
		},
	}} {
		event := test.input.BeatEvent()
		assert.Equal(t, test.output, event.Fields)
	}
}
