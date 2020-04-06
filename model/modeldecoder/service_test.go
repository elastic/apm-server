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

package modeldecoder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model/metadata"
)

const (
	serviceName, serviceVersion, serviceEnvironment, serviceNodeName = "myservice", "5.1.3", "staging", "serviceABC"
	langName, langVersion                                            = "ecmascript", "8"
	rtName, rtVersion                                                = "node", "8.0.0"
	fwName, fwVersion                                                = "Express", "1.2.3"
	agentName, agentVersion                                          = "elastic-node", "1.0.0"
)

func TestServiceDecode(t *testing.T) {
	serviceName := "myService"
	for _, test := range []struct {
		input map[string]interface{}
		s     metadata.Service
	}{
		{input: nil},
		{
			input: map[string]interface{}{"name": 1234},
			s: metadata.Service{
				Language:  metadata.Language{},
				Runtime:   metadata.Runtime{},
				Framework: metadata.Framework{},
				Agent:     metadata.Agent{},
			},
		},
		{
			input: map[string]interface{}{
				"name":        serviceName,
				"version":     "5.1.3",
				"environment": "staging",
				"language": map[string]interface{}{
					"name":    "ecmascript",
					"version": "8",
				},
				"runtime": map[string]interface{}{
					"name":    "node",
					"version": "8.0.0",
				},
				"framework": map[string]interface{}{
					"name":    "Express",
					"version": "1.2.3",
				},
				"agent": map[string]interface{}{
					"name":    "elastic-node",
					"version": "1.0.0",
				},
			},
			s: metadata.Service{
				Name:        serviceName,
				Version:     serviceVersion,
				Environment: serviceEnvironment,
				Language: metadata.Language{
					Name:    langName,
					Version: langVersion,
				},
				Runtime: metadata.Runtime{
					Name:    rtName,
					Version: rtVersion,
				},
				Framework: metadata.Framework{
					Name:    fwName,
					Version: fwVersion,
				},
				Agent: metadata.Agent{
					Name:    agentName,
					Version: agentVersion,
				},
			},
		},
	} {
		var service metadata.Service
		decodeService(test.input, false, &service)
		assert.Equal(t, test.s, service)
	}
}
