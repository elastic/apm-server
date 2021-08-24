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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
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

	for _, test := range []struct {
		input  APMEvent
		fields common.MapStr
		output common.MapStr
	}{
		{
			input: APMEvent{
				ECSVersion: "1.0.0",
				Agent: Agent{
					Name:    agentName,
					Version: agentVersion,
				},
				Observer:  Observer{Type: "apm-server"},
				Container: Container{ID: containerID},
				Service: Service{
					Name: serviceName,
					Node: ServiceNode{Name: serviceNodeName},
				},
				Host: Host{
					Hostname: hostname,
					Name:     host,
				},
				Client:      Client{Domain: "client.domain"},
				Destination: Destination{Address: destinationAddress, Port: destinationPort},
				Process:     Process{Pid: pid},
				User:        User{ID: uid, Email: mail},
				Event:       Event{Outcome: outcome},
				Session:     Session{ID: "session_id"},
				URL:         URL{Original: "url"},
				Labels:      common.MapStr{"a": "b", "c": 123},
				Message:     "bottle",
				Transaction: &Transaction{},
				Timestamp:   time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6, time.FixedZone("+0100", 3600)),
<<<<<<< HEAD
=======
				Processor:   Processor{Name: "processor_name", Event: "processor_event"},
				Trace:       Trace{ID: traceID},
>>>>>>> cb6b2dab (Introduce model.Processor (#5984))
			},
			output: common.MapStr{
				// common fields
				"ecs":       common.MapStr{"version": "1.0.0"},
				"agent":     common.MapStr{"version": "1.0.0", "name": "elastic-node"},
				"observer":  common.MapStr{"type": "apm-server"},
				"container": common.MapStr{"id": containerID},
				"host":      common.MapStr{"hostname": hostname, "name": host},
				"process":   common.MapStr{"pid": pid},
				"service": common.MapStr{
					"name": "myservice",
					"node": common.MapStr{"name": serviceNodeName},
				},
				"user":   common.MapStr{"id": "12321", "email": "user@email.com"},
				"client": common.MapStr{"domain": "client.domain"},
				"destination": common.MapStr{
					"address": destinationAddress,
					"ip":      destinationAddress,
					"port":    destinationPort,
				},
				"source":  common.MapStr{"domain": "client.domain"},
				"event":   common.MapStr{"outcome": outcome},
				"session": common.MapStr{"id": "session_id"},
				"url":     common.MapStr{"original": "url"},
				"labels": common.MapStr{
					"a": "b",
					"c": 123,
				},
				"message": "bottle",
<<<<<<< HEAD

				// fields related to APMEvent.Transaction
=======
				"trace": common.MapStr{
					"id": traceID,
				},
>>>>>>> cb6b2dab (Introduce model.Processor (#5984))
				"processor": common.MapStr{
					"name":  "processor_name",
					"event": "processor_event",
				},

				// fields related to APMEvent.Transaction
				"timestamp": common.MapStr{"us": int64(1546525024908596)},
				"transaction": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"sampled":  false,
					"type":     "",
					"id":       "",
				},
			},
		},
	} {
		event := test.input.BeatEvent(context.Background())
		assert.Equal(t, test.output, event.Fields)
	}
}
