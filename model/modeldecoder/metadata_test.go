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
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestDecodeMetadata(t *testing.T) {
	pid := 1234
	host := "host"
	serviceName, serviceNodeName := "myservice", "serviceABC"
	uid := "12321"
	mail := "user@email.com"
	agentName := "elastic-node"

	input := map[string]interface{}{
		"process": map[string]interface{}{
			"pid": 1234.0,
		},
		"service": map[string]interface{}{
			"name": "myservice",
			"node": map[string]interface{}{
				"configured_name": serviceNodeName,
			},
			"agent": map[string]interface{}{
				"name":    agentName,
				"version": agentVersion,
			},
		},
		"system": map[string]interface{}{
			"hostname": host,
		},
		"user": map[string]interface{}{
			"id": uid, "email": mail,
		},
		"labels": map[string]interface{}{
			"k": "v", "n": 1, "f": 1.5, "b": false,
		},
	}
	output, err := DecodeMetadata(input, false)
	require.NoError(t, err)
	assert.Equal(t, &metadata.Metadata{
		Service: &metadata.Service{
			Name:  &serviceName,
			Agent: metadata.Agent{Name: &agentName, Version: &agentVersion},
			Node:  metadata.ServiceNode{Name: &serviceNodeName},
		},
		System:  &metadata.System{DetectedHostname: &host},
		Process: &metadata.Process{Pid: pid},
		User:    &metadata.User{Id: &uid, Email: &mail},
		Labels:  common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
	}, output)
}

func TestDecodeMetadataInvalid(t *testing.T) {
	_, err := DecodeMetadata(nil, false)
	require.EqualError(t, err, "failed to validate metadata: error validating JSON: input missing")

	_, err = DecodeMetadata("", false)
	require.EqualError(t, err, "failed to validate metadata: error validating JSON: invalid input type")

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"service": map[string]interface{}{
			"agent": map[string]interface{}{},
			"name":  "name",
		},
	}
	_, err = DecodeMetadata(baseInput, false)
	require.NoError(t, err)

	for _, test := range []struct {
		input map[string]interface{}
	}{
		{
			input: map[string]interface{}{"service": 123},
		},
		{
			input: map[string]interface{}{"system": 123},
		},
		{
			input: map[string]interface{}{"process": 123},
		},
		{
			input: map[string]interface{}{"user": 123},
		},
	} {
		input := make(map[string]interface{})
		for k, v := range baseInput {
			input[k] = v
		}
		for k, v := range test.input {
			if v == nil {
				delete(input, k)
			} else {
				input[k] = v
			}
		}
		_, err := DecodeMetadata(input, false)
		assert.Error(t, err)
		t.Logf("%s", err)
	}

}
