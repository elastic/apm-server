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
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.elastic.co/apm/model"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests"
)

func TestMetadata_Set(t *testing.T) {
	pid := 1234
	host := "host"
	containerID := "container-123"
	serviceName, serviceNodeName := "myservice", "serviceABC"
	uid := "12321"
	mail := "user@email.com"
	agentName := "elastic-node"

	for _, test := range []struct {
		input  Metadata
		fields common.MapStr
		output common.MapStr
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
				System:  System{DetectedHostname: host, Container: Container{ID: containerID}},
				Process: Process{Pid: pid},
				User:    User{ID: uid, Email: mail},
			},
			fields: common.MapStr{
				"foo": "bar",
				"user": common.MapStr{
					"email": "override@email.com",
				},
			},
			output: common.MapStr{
				"foo":       "bar",
				"agent":     common.MapStr{"version": "1.0.0", "name": "elastic-node"},
				"container": common.MapStr{"id": containerID},
				"host":      common.MapStr{"hostname": host, "name": host},
				"process":   common.MapStr{"pid": pid},
				"service": common.MapStr{
					"name": "myservice",
					"node": common.MapStr{"name": serviceNodeName},
				},
				"user": common.MapStr{"id": "12321", "email": "user@email.com"},
			},
		},
		{
			input: Metadata{
				Service: Service{},
				System:  System{DetectedHostname: host, Container: Container{ID: containerID}},
			},
			fields: common.MapStr{},
			output: common.MapStr{
				"host":      common.MapStr{"hostname": host, "name": host},
				"container": common.MapStr{"id": containerID},
				"service":   common.MapStr{"node": common.MapStr{"name": containerID}}},
		},
		{
			input: Metadata{
				Service: Service{},
				System:  System{DetectedHostname: host},
			},
			fields: common.MapStr{},
			output: common.MapStr{
				"host":    common.MapStr{"hostname": host, "name": host},
				"service": common.MapStr{"node": common.MapStr{"name": host}}},
		},
	} {
		assert.Equal(t, test.output, test.input.Set(test.fields))
	}
}

func BenchmarkMetadataSet(b *testing.B) {
	test := func(b *testing.B, name string, input Metadata) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			out := make(common.MapStr)
			for i := 0; i < b.N; i++ {
				input.Set(out)
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

func BenchmarkMetadataDecode(b *testing.B) {
	maxEventSize := 307200
	input := []byte(`{"metadata":{"process":{"pid":1234,"title":"/usr/lib/jvm/java-10-openjdk-amd64/bin/java","ppid":1,"argv":["-v"]},"system":{"architecture":"amd64","detected_hostname":"8ec7ceb99074","configured_hostname":"host1","platform":"Linux","container":{"id":"8ec7ceb990749e79b37f6dc6cd3628633618d6ce412553a552a0fa6b69419ad4"},"kubernetes":{"namespace":"default","pod":{"uid":"b17f231da0ad128dc6c6c0b2e82f6f303d3893e3","name":"instrumented-java-service"},"node":{"name":"node-name"}}},"service":{"name":"1234_service-12a3","version":"4.3.0","node":{"configured_name":"8ec7ceb990749e79b37f6dc6cd3628633618d6ce412553a552a0fa6b69419ad4"},"environment":"production","language":{"name":"Java","version":"10.0.2"},"agent":{"version":"1.10.0","name":"java","ephemeral_id":"e71be9ac-93b0-44b9-a997-5638f6ccfc36"},"framework":{"name":"spring","version":"5.0.0"},"runtime":{"name":"Java","version":"10.0.2"}},"labels":{"group":"experimental","ab_testing":true,"segment":5}}}`)

	reader := bytes.NewReader(input)
	var dec *decoder.NDJSONStreamDecoder
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		reader.Reset(input)
		dec = decoder.NewNDJSONStreamDecoder(reader, maxEventSize)
		b.StartTimer()

		for !dec.IsEOF() {
			var span model.Span
			if err := dec.Decode(&span); err != nil && !dec.IsEOF() {
				panic(err.Error())
			}
		}
	}
}
