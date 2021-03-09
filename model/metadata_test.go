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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/tests"
)

func TestMetadata_Set(t *testing.T) {
	pid := 1234
	host := "host"
	hostname := "hostname"
	containerID := "container-123"
	serviceName, serviceNodeName := "myservice", "serviceABC"
	uid := "12321"
	mail := "user@email.com"
	agentName := "elastic-node"

	for _, test := range []struct {
		input  Metadata
		fields mapStr
		output mapStr
	}{
		{
			input: Metadata{
				Service: Service{
					Name: serviceName,
					Node: ServiceNode{Name: serviceNodeName},
					Agent: Agent{
						Name:    agentName,
						Version: agentVersion,
					},
				},
				System: System{
					ConfiguredHostname: host,
					DetectedHostname:   hostname,
					Container:          Container{ID: containerID}},
				Process: Process{Pid: pid},
				User:    User{ID: uid, Email: mail},
			},
			fields: mapStr{
				"foo": "bar",
				"user": common.MapStr{
					"email": "override@email.com",
				},
			},
			output: mapStr{
				"foo":       "bar",
				"agent":     common.MapStr{"version": "1.0.0", "name": "elastic-node"},
				"container": common.MapStr{"id": containerID},
				"host":      common.MapStr{"hostname": hostname, "name": host},
				"process":   common.MapStr{"pid": pid},
				"service": common.MapStr{
					"name": "myservice",
					"node": common.MapStr{"name": serviceNodeName},
				},
				"user": common.MapStr{"id": "12321", "email": "user@email.com"},
			},
		},
	} {
		test.input.set(&test.fields, nil)
		assert.Equal(t, test.output, test.fields)
	}
}

func BenchmarkMetadataSet(b *testing.B) {
	test := func(b *testing.B, name string, input Metadata) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var out mapStr
			for i := 0; i < b.N; i++ {
				input.set(&out, nil)
				for k := range out {
					delete(out, k)
				}
			}
		})
	}

	test(b, "minimal", Metadata{
		Service: Service{
			Name:    "foo",
			Version: "1.0",
		},
	})
	test(b, "full", Metadata{
		Service: Service{
			Name:        "foo",
			Version:     "1.0",
			Environment: "production",
			Node:        ServiceNode{Name: "foo-bar"},
			Language:    Language{Name: "go", Version: "++"},
			Runtime:     Runtime{Name: "gc", Version: "1.0"},
			Framework:   Framework{Name: "never", Version: "again"},
			Agent:       Agent{Name: "go", Version: "2.0"},
		},
		Process: Process{
			Pid:   123,
			Ppid:  tests.IntPtr(122),
			Title: "case",
			Argv:  []string{"apm-server"},
		},
		System: System{
			DetectedHostname:   "detected",
			ConfiguredHostname: "configured",
			Architecture:       "x86_64",
			Platform:           "linux",
			IP:                 net.ParseIP("10.1.1.1"),
			Container:          Container{ID: "docker"},
			Kubernetes: Kubernetes{
				Namespace: "system",
				NodeName:  "node01",
				PodName:   "pet",
				PodUID:    "cattle",
			},
		},
		User: User{
			ID:    "123",
			Email: "me@example.com",
			Name:  "bob",
		},
		UserAgent: UserAgent{
			Original: "user-agent",
		},
		Client: Client{
			IP: net.ParseIP("10.1.1.2"),
		},
		Labels: common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
	})
}
