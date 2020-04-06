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

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	version, environment    = "5.1.3", "staging"
	langName, langVersion   = "ecmascript", "8"
	rtName, rtVersion       = "node", "8.0.0"
	fwName, fwVersion       = "Express", "1.2.3"
	agentName, agentVersion = "elastic-node", "1.0.0"
)

func TestServiceTransform(t *testing.T) {
	serviceName, serviceNodeName := "myService", "abc"

	tests := []struct {
		Service               Service
		ContainerID, HostName string
		Fields                common.MapStr
		AgentFields           common.MapStr
	}{
		{
			Service:     Service{},
			AgentFields: nil,
			Fields:      nil,
		}, {
			Service:     Service{},
			ContainerID: "foo",
			HostName:    "bar",
			AgentFields: nil,
			Fields:      common.MapStr{"node": common.MapStr{"name": "foo"}},
		}, {
			Service:     Service{},
			HostName:    "bar",
			AgentFields: nil,
			Fields:      common.MapStr{"node": common.MapStr{"name": "bar"}},
		},
		{
			Service: Service{
				Name:        serviceName,
				Version:     version,
				Environment: environment,
				Language: Language{
					Name:    langName,
					Version: langVersion,
				},
				Runtime: Runtime{
					Name:    rtName,
					Version: rtVersion,
				},
				Framework: Framework{
					Name:    fwName,
					Version: fwVersion,
				},
				Agent: Agent{
					Name:    agentName,
					Version: agentVersion,
				},
				Node: ServiceNode{Name: serviceNodeName},
			},
			ContainerID: "foo",
			HostName:    "bar",
			AgentFields: common.MapStr{
				"name":    "elastic-node",
				"version": "1.0.0",
			},
			Fields: common.MapStr{
				"name":        "myService",
				"version":     "5.1.3",
				"environment": "staging",
				"language": common.MapStr{
					"name":    "ecmascript",
					"version": "8",
				},
				"runtime": common.MapStr{
					"name":    "node",
					"version": "8.0.0",
				},
				"framework": common.MapStr{
					"name":    "Express",
					"version": "1.2.3",
				},
				"node": common.MapStr{"name": serviceNodeName},
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.Fields, test.Service.Fields(test.ContainerID, test.HostName))
		assert.Equal(t, test.AgentFields, test.Service.AgentFields())
	}
}
