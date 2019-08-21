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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests/approvals"
	"github.com/elastic/apm-server/utility"
)

func TestSystem(t *testing.T) {
	host, configured, detected := "host", "custom hostname", "detected hostname"
	arch, platform, ip, containerID, namespace := "amd", "osx", "127.0.0.1", "1234", "staging"
	empty := ""
	nodename, podname, podUID := "a.node", "a.pod", "b.podID"

	inpErr := errors.New("some error")
	for name, test := range map[string]struct {
		input         interface{}
		inputErr, err error
		s             *System
	}{
		"nil":         {input: nil, err: nil, s: nil},
		"inputError":  {input: nil, inputErr: inpErr, err: inpErr, s: nil},
		"invalidType": {input: "", err: errors.New("invalid type for system"), s: nil},
		"empty":       {input: map[string]interface{}{}, s: &System{}},
		"fetchError": {
			input: map[string]interface{}{"hostname": 1},
			err:   utility.ErrFetch,
			s:     &System{DetectedHostname: nil, Architecture: nil, Platform: nil, IP: nil},
		},
		"empty ip": {
			input: map[string]interface{}{"ip": ""},
			s:     &System{IP: &empty},
		},
		"hostname": {
			input: map[string]interface{}{"hostname": host},
			s:     &System{DetectedHostname: &host},
		},
		"detected hostname": {
			// in practice either hostname or detected_hostname should be sent, but in theory both can be sent, so
			// testing that the server does process the proper one in such a case.
			input: map[string]interface{}{
				"hostname": host, "detected_hostname": detected,
			},
			s: &System{DetectedHostname: &detected},
		},
		"ignored hostname": {
			// in practice either hostname or configured_hostname should be sent, but in theory both can be sent, so
			// testing that the server does process the proper one in such a case.
			input: map[string]interface{}{
				"hostname": host, "configured_hostname": configured,
			},
			s: &System{ConfiguredHostname: &configured},
		},
		"k8s nodename with hostname": {
			input: map[string]interface{}{
				"kubernetes": map[string]interface{}{"node": map[string]interface{}{"name": nodename}},
				"hostname":   host,
			},
			s: &System{Kubernetes: &Kubernetes{NodeName: &nodename}, DetectedHostname: &host},
		},
		"k8s nodename with configured hostname": {
			input: map[string]interface{}{
				"kubernetes": map[string]interface{}{"node": map[string]interface{}{"name": nodename}},
				"hostname":   host, "configured_hostname": configured,
			},
			s: &System{Kubernetes: &Kubernetes{NodeName: &nodename}, ConfiguredHostname: &configured},
		},
		"k8s nodename with detected hostname": {
			input: map[string]interface{}{
				"kubernetes": map[string]interface{}{"node": map[string]interface{}{"name": nodename}},
				"hostname":   host, "detected_hostname": detected,
			},
			s: &System{Kubernetes: &Kubernetes{NodeName: &nodename}, DetectedHostname: &detected},
		},
		"k8s podname": {
			input: map[string]interface{}{
				"kubernetes":        map[string]interface{}{"pod": map[string]interface{}{"name": podname}},
				"detected_hostname": detected,
			},
			s: &System{Kubernetes: &Kubernetes{PodName: &podname}, DetectedHostname: &detected},
		},
		"k8s podUID": {
			input: map[string]interface{}{
				"kubernetes":        map[string]interface{}{"pod": map[string]interface{}{"uid": podUID}},
				"detected_hostname": detected,
			},
			s: &System{Kubernetes: &Kubernetes{PodUID: &podUID}, DetectedHostname: &detected},
		},
		"k8s_namespace": {
			input: map[string]interface{}{
				"kubernetes":        map[string]interface{}{"namespace": namespace},
				"detected_hostname": detected,
			},
			s: &System{Kubernetes: &Kubernetes{Namespace: &namespace}, DetectedHostname: &detected},
		},
		"k8s podname with configured hostname": {
			input: map[string]interface{}{
				"kubernetes":          map[string]interface{}{"pod": map[string]interface{}{"name": podname}},
				"detected_hostname":   detected,
				"configured_hostname": configured,
			},
			s: &System{Kubernetes: &Kubernetes{PodName: &podname}, DetectedHostname: &detected, ConfiguredHostname: &configured},
		},
		"k8s podUID with configured hostname": {
			input: map[string]interface{}{
				"kubernetes":          map[string]interface{}{"pod": map[string]interface{}{"uid": podUID}},
				"detected_hostname":   detected,
				"configured_hostname": configured,
			},
			s: &System{Kubernetes: &Kubernetes{PodUID: &podUID}, DetectedHostname: &detected, ConfiguredHostname: &configured},
		},
		"k8s namespace with configured hostname": {
			input: map[string]interface{}{
				"kubernetes":          map[string]interface{}{"namespace": namespace},
				"detected_hostname":   detected,
				"configured_hostname": configured,
			},
			s: &System{Kubernetes: &Kubernetes{Namespace: &namespace}, DetectedHostname: &detected, ConfiguredHostname: &configured},
		},
		"k8s empty": {
			input: map[string]interface{}{
				"kubernetes":          map[string]interface{}{},
				"detected_hostname":   detected,
				"configured_hostname": configured,
			},
			s: &System{Kubernetes: &Kubernetes{}, DetectedHostname: &detected, ConfiguredHostname: &configured},
		},
		"full hostname info": {
			input: map[string]interface{}{
				"detected_hostname":   detected,
				"configured_hostname": configured,
			},
			s: &System{DetectedHostname: &detected, ConfiguredHostname: &configured},
		},
		"full": {
			input: map[string]interface{}{
				"platform":     platform,
				"architecture": arch,
				"ip":           ip,
				"container":    map[string]interface{}{"id": containerID},
				"kubernetes": map[string]interface{}{
					"namespace": namespace,
					"node":      map[string]interface{}{"name": nodename},
					"pod": map[string]interface{}{
						"uid":  podUID,
						"name": podname,
					},
				},
				"configured_hostname": configured,
				"detected_hostname":   detected,
			},
			err: nil,
			s: &System{
				DetectedHostname:   &detected,
				ConfiguredHostname: &configured,
				Architecture:       &arch,
				Platform:           &platform,
				IP:                 &ip,
				Container:          &Container{ID: containerID},
				Kubernetes:         &Kubernetes{Namespace: &namespace, NodeName: &nodename, PodName: &podname, PodUID: &podUID},
			},
		},
	} {

		t.Run(name, func(t *testing.T) {
			system, err := DecodeSystem(test.input, test.inputErr)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.s, system)

			if test.err == nil {
				resultName := fmt.Sprintf("test_approved_system/transform_%s", strings.ReplaceAll(name, " ", "_"))
				resultJSON, err := json.Marshal(system.fields())
				require.NoError(t, err)
				approvals.AssertApproveResult(t, resultName, resultJSON)
			}
		})
	}
}
