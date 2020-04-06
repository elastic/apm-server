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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/utility"
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
		input       interface{}
		err, inpErr error
		s           *metadata.Service
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: "", err: errors.New("invalid type for service"), s: nil},
		{
			input: map[string]interface{}{"name": 1234},
			err:   utility.ErrFetch,
			s: &metadata.Service{
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
			s: &metadata.Service{
				Name:        tests.StringPtr(serviceName),
				Version:     tests.StringPtr(serviceVersion),
				Environment: tests.StringPtr(serviceEnvironment),
				Language: metadata.Language{
					Name:    tests.StringPtr(langName),
					Version: tests.StringPtr(langVersion),
				},
				Runtime: metadata.Runtime{
					Name:    tests.StringPtr(rtName),
					Version: tests.StringPtr(rtVersion),
				},
				Framework: metadata.Framework{
					Name:    tests.StringPtr(fwName),
					Version: tests.StringPtr(fwVersion),
				},
				Agent: metadata.Agent{
					Name:    tests.StringPtr(agentName),
					Version: tests.StringPtr(agentVersion),
				},
			},
		},
	} {
		service, out := decodeService(test.input, false, test.inpErr)
		assert.Equal(t, test.s, service)
		assert.Equal(t, test.err, out)
	}
}
