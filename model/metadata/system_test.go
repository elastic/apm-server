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

func TestSystemTransform(t *testing.T) {

	architecture := "x64"
	hostname, name := "a.b.com", "foo"
	platform := "darwin"
	ip := "127.0.0.1"
	empty := ""
	nodename := "a.node"
	podname := "a.pod"

	tests := []struct {
		System System
		Output common.MapStr
	}{
		{
			System: System{},
			Output: common.MapStr{},
		},
		{
			System: System{
				IP: &empty,
			},
			Output: common.MapStr{},
		},
		{
			System: System{
				Architecture: &architecture,
				Hostname:     &hostname,
				Name:         &name,
				Platform:     &platform,
				IP:           &ip,
			},
			Output: common.MapStr{
				"architecture": architecture,
				"hostname":     hostname,
				"name":         name,
				"ip":           ip,
				"os": common.MapStr{
					"platform": platform,
				},
			},
		},
		{
			System: System{
				Architecture: &architecture,
				Hostname:     &hostname,
			},
			Output: common.MapStr{
				"architecture": architecture,
				"hostname":     hostname,
				"name":         hostname,
			},
		},
		// nodename and podname
		{
			System: System{
				Hostname: &hostname,
				Kubernetes: &Kubernetes{
					NodeName: &nodename,
					PodName:  &podname,
				},
			},
			Output: common.MapStr{
				"hostname": nodename,
				"name":     nodename,
			},
		},
		// podname
		{
			System: System{
				Hostname: &hostname,
				Name:     &name,
				Kubernetes: &Kubernetes{
					PodName: &podname,
				},
			},
			Output: common.MapStr{"name": name},
		},
		// poduid
		{
			System: System{
				Hostname: &hostname,
				Kubernetes: &Kubernetes{
					PodUID: &podname, // any string
				},
			},
			Output: common.MapStr{},
		},
		// namespace
		{
			System: System{
				Hostname: &hostname,
				Kubernetes: &Kubernetes{
					Namespace: &podname, // any string
				},
			},
			Output: common.MapStr{},
		},
		// non-nil kubernetes, currently not possible via intake
		{
			System: System{
				Hostname:   &hostname,
				Kubernetes: &Kubernetes{},
			},
			Output: common.MapStr{
				"hostname": hostname,
				"name":     hostname,
			},
		},
	}

	for _, test := range tests {
		output := test.System.fields()
		assert.Equal(t, test.Output, output)
	}
}

func TestSystemDecode(t *testing.T) {
	host, name, arch, platform, ip := "host", "custom hostname", "amd", "osx", "127.0.0.1"
	inpErr := errors.New("some error")
	for _, test := range []struct {
		input         interface{}
		inputErr, err error
		s             *System
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inputErr: inpErr, err: inpErr, s: nil},
		{input: "", err: errors.New("invalid type for system"), s: nil},
		{
			input: map[string]interface{}{"hostname": 1},
			err:   utility.ErrFetch,
			s:     &System{Hostname: nil, Architecture: nil, Platform: nil, IP: nil},
		},
		{
			input: map[string]interface{}{
				"hostname": host, "architecture": arch, "platform": platform, "ip": ip,
			},
			err: nil,
			s:   &System{Hostname: &host, Architecture: &arch, Platform: &platform, IP: &ip},
		},
		{
			input: map[string]interface{}{
				"hostname": host, "name": name,
			},
			err: nil,
			s:   &System{Hostname: &host, Name: &name},
		},
	} {
		sys, err := DecodeSystem(test.input, test.inputErr)
		assert.Equal(t, test.s, sys)
		assert.Equal(t, test.err, err)
	}
}
