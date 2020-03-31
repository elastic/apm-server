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
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestDecodeMetadata(t *testing.T) {
	pid := 1234
	host := "host"
	serviceName, serviceNodeName := "myservice", "serviceABC"
	uid := "12321"
	mail := "user@email.com"
	agentName := "elastic-node"

	for _, test := range []struct {
		input  interface{}
		err    string
		output Metadata
	}{
		{
			input: nil,
			err:   "failed to validate metadata: input missing",
		},
		{
			input: "it doesn't work on strings",
			err:   "failed to validate metadata: invalid input type",
		},
		{
			input: map[string]interface{}{"service": 123},
			err:   "invalid type for service",
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   "invalid type for system",
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   "invalid type for process",
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   "invalid type for user",
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   "invalid type for user",
		},
		{
			input: map[string]interface{}{
				"process": map[string]interface{}{
					"pid": 1234.0,
				},
				"service": map[string]interface{}{
					"name": "myservice",
					"node": map[string]interface{}{
						"configured_name": serviceNodeName},
					"agent": map[string]interface{}{
						"name":    agentName,
						"version": agentVersion},
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
			},
			output: Metadata{
				Service: &Service{
					Name:  &serviceName,
					Agent: Agent{Name: &agentName, Version: &agentVersion},
					Node:  ServiceNode{Name: &serviceNodeName},
				},
				System:  &System{DetectedHostname: &host},
				Process: &Process{Pid: pid},
				User:    &User{Id: &uid, Email: &mail},
				Labels:  common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
			},
		},
	} {
		output, err := DecodeMetadata(test.input, false)
		if test.err != "" {
			assert.EqualError(t, err, test.err)
			assert.Nil(t, output)
		} else {
			assert.NoError(t, err)
			if test.input == nil {
				assert.Nil(t, output)
			} else {
				require.NotNil(t, output)
				assert.Equal(t, test.output, *output)
			}
		}
	}

}

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
				Service: &Service{
					Name: &serviceName,
					Node: ServiceNode{Name: &serviceNodeName},
					Agent: Agent{
						Name:    &agentName,
						Version: &agentVersion,
					},
				},
				System:  &System{DetectedHostname: &host, Container: &Container{ID: containerID}},
				Process: &Process{Pid: pid},
				User:    &User{Id: &uid, Email: &mail},
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
				Service: &Service{},
				System:  &System{DetectedHostname: &host, Container: &Container{ID: containerID}},
			},
			fields: common.MapStr{},
			output: common.MapStr{
				"host":      common.MapStr{"hostname": host, "name": host},
				"container": common.MapStr{"id": containerID},
				"service":   common.MapStr{"node": common.MapStr{"name": containerID}}},
		},
		{
			input: Metadata{
				Service: &Service{},
				System:  &System{DetectedHostname: &host},
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
