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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	for _, test := range []struct {
		input  APMEvent
		fields common.MapStr
		output common.MapStr
	}{
		{
			input: APMEvent{
				Agent: Agent{
					Name:    agentName,
					Version: agentVersion,
				},
				Container: Container{ID: containerID},
				Service: Service{
					Name: serviceName,
					Node: ServiceNode{Name: serviceNodeName},
				},
				Host: Host{
					Hostname: hostname,
					Name:     host,
				},
				Client:  Client{Domain: "client.domain"},
				Process: Process{Pid: pid},
				User:    User{ID: uid, Email: mail},
				Labels:  common.MapStr{"a": "a1", "b": "b1"},
				Transaction: &Transaction{
					Labels: common.MapStr{"b": "b2", "c": "c2"},
				},
			},
			output: common.MapStr{
				// common fields
				"agent":     common.MapStr{"version": "1.0.0", "name": "elastic-node"},
				"container": common.MapStr{"id": containerID},
				"host":      common.MapStr{"hostname": hostname, "name": host},
				"process":   common.MapStr{"pid": pid},
				"service": common.MapStr{
					"name": "myservice",
					"node": common.MapStr{"name": serviceNodeName},
				},
				"user":   common.MapStr{"id": "12321", "email": "user@email.com"},
				"client": common.MapStr{"domain": "client.domain"},
				"source": common.MapStr{"domain": "client.domain"},
				"labels": common.MapStr{
					"a": "a1",
					"b": "b2",
					"c": "c2",
				},

				// fields related to APMEvent.Transaction
				"event": common.MapStr{"outcome": ""},
				"processor": common.MapStr{
					"name":  "transaction",
					"event": "transaction",
				},
				"transaction": common.MapStr{
					"duration": common.MapStr{"us": 0},
					"sampled":  false,
					"type":     "",
					"id":       "",
				},
			},
		},
	} {
		events := test.input.appendBeatEvent(context.Background(), nil)
		require.Len(t, events, 1)
		assert.Equal(t, test.output, events[0].Fields)
	}
}
