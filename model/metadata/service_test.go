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

	tests := []struct {
		Service Service
		Output  common.MapStr
	}{
		{
			Service: Service{},
			Output: common.MapStr{
				"agent": common.MapStr{
					"name":    "",
					"version": "",
				},
				"name": "",
			},
		},
		{
			Service: Service{
				Name:        "myService",
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
					Name:    agentName,
					Version: agentVersion,
				},
			},
			Output: common.MapStr{
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
				"agent": common.MapStr{
					"name":    "elastic-node",
					"version": "1.0.0",
				},
			},
		},
	}

	for _, test := range tests {
		output := test.Service.fields()
		assert.Equal(t, test.Output, output)
	}
}

func TestServiceDecode(t *testing.T) {
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Service
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: "", err: errors.New("Invalid type for service"), s: nil},
		{
			input: map[string]interface{}{"name": 1234},
			err:   errors.New("Error fetching field"),
			s: &Service{
				Name: "", Version: nil, Environment: nil,
				Language:  Language{Name: nil, Version: nil},
				Runtime:   Runtime{Name: nil, Version: nil},
				Framework: Framework{Name: nil, Version: nil},
				Agent:     Agent{Name: "", Version: ""},
			},
		},
		{
			input: map[string]interface{}{
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
				"agent": common.MapStr{
					"name":    "elastic-node",
					"version": "1.0.0",
				},
			},
			err: nil,
			s: &Service{
				Name:        "myService",
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
					Name:    agentName,
					Version: agentVersion,
				},
			},
		},
	} {
		service, out := DecodeService(test.input, test.inpErr)
		assert.Equal(t, test.s, service)
		assert.Equal(t, test.err, out)
	}
}
