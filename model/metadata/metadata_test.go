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
		err    error
		output *Metadata
	}{
		{
			input: nil,
			err:   nil,
		},
		{
			input: "it doesn't work on strings",
			err:   errors.New("invalid type for metadata"),
		},
		{
			input: map[string]interface{}{"service": 123},
			err:   errors.New("invalid type for service"),
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   errors.New("invalid type for system"),
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   errors.New("invalid type for process"),
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   errors.New("invalid type for user"),
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   errors.New("invalid type for user"),
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
			output: NewMetadata(
				&Service{
					Name:  &serviceName,
					Agent: Agent{Name: &agentName, Version: &agentVersion},
					node:  node{name: &serviceNodeName},
				},
				&System{DetectedHostname: &host},
				&Process{Pid: pid},
				&User{Id: &uid, Email: &mail},
				common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
			),
		},
	} {
		metadata, err := DecodeMetadata(test.input)
		assert.Equal(t, test.err, err)
		assert.Equal(t, test.output, metadata)
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
		input  *Metadata
		fields common.MapStr
		output common.MapStr
	}{
		{
			input: NewMetadata(
				&Service{
					Name: &serviceName,
					node: node{name: &serviceNodeName},
					Agent: Agent{
						Name:    &agentName,
						Version: &agentVersion,
					},
				},
				&System{DetectedHostname: &host, Container: &Container{ID: containerID}},
				&Process{Pid: pid},
				&User{Id: &uid, Email: &mail},
				nil,
			),
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
			input: NewMetadata(&Service{},
				&System{DetectedHostname: &host, Container: &Container{ID: containerID}},
				nil, nil, nil),
			fields: common.MapStr{},
			output: common.MapStr{
				"host":      common.MapStr{"hostname": host, "name": host},
				"container": common.MapStr{"id": containerID},
				"service":   common.MapStr{"node": common.MapStr{"name": containerID}}},
		},
		{
			input:  NewMetadata(&Service{}, &System{DetectedHostname: &host}, nil, nil, nil),
			fields: common.MapStr{},
			output: common.MapStr{
				"host":    common.MapStr{"hostname": host, "name": host},
				"service": common.MapStr{"node": common.MapStr{"name": host}}},
		},
	} {
		assert.Equal(t, test.output, test.input.Set(test.fields))
	}
}
