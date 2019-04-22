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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

var (
	version, environment    = "5.1.3", "staging"
	langName, langVersion   = "ecmascript", "8"
	rtName, rtVersion       = "node", "8.0.0"
	fwName, fwVersion       = "Express", "1.2.3"
	agentName, agentVersion = "elastic-node", "1.0.0"
)

func TestServiceTransform(t *testing.T) {
	serviceName := "myService"

	tests := []struct {
		Service     Service
		Fields      common.MapStr
		AgentFields common.MapStr
	}{
		{
			Service:     Service{},
			AgentFields: common.MapStr{},
			Fields:      common.MapStr{},
		},
		{
			Service: Service{
				Name:        &serviceName,
				Version:     &version,
				Environment: &environment,
				Language: Language{
					Name:    &langName,
					Version: &langVersion,
				},
				Runtime: Runtime{
					Name:    &rtName,
					Version: &rtVersion,
				},
				Framework: Framework{
					Name:    &fwName,
					Version: &fwVersion,
				},
				Agent: Agent{
					Name:    &agentName,
					Version: &agentVersion,
				},
			},
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
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.Fields, test.Service.Fields())
		assert.Equal(t, test.AgentFields, test.Service.AgentFields())
	}
}

func TestServiceDecode(t *testing.T) {
	serviceName := "myService"
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Service
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: "", err: errors.New("invalid type for service"), s: nil},
		{
			input: map[string]interface{}{"name": 1234},
			err:   utility.ErrFetch,
			s: &Service{
				Language:  Language{},
				Runtime:   Runtime{},
				Framework: Framework{},
				Agent:     Agent{},
			},
		},
		{
			input: map[string]interface{}{
				"name":        serviceName,
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
				"agent": common.MapStr{
					"name":    "elastic-node",
					"version": "1.0.0",
				},
			},
			err: nil,
			s: &Service{
				Name:        &serviceName,
				Version:     &version,
				Environment: &environment,
				Language: Language{
					Name:    &langName,
					Version: &langVersion,
				},
				Runtime: Runtime{
					Name:    &rtName,
					Version: &rtVersion,
				},
				Framework: Framework{
					Name:    &fwName,
					Version: &fwVersion,
				},
				Agent: Agent{
					Name:    &agentName,
					Version: &agentVersion,
				},
			},
		},
	} {
		service, out := DecodeService(test.input, test.inpErr)
		assert.Equal(t, test.s, service)
		assert.Equal(t, test.err, out)
	}
}
