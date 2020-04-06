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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	pid          = 1234
	ppid         = 4567
	processTitle = "bobsyouruncle"

	detectedHostname   = "detected_hostname"
	configuredHostname = "configured_hostname"
	systemArchitecture = "x86_64"
	systemPlatform     = "linux"
	systemIP           = "192.168.0.1"

	containerID         = "container-123"
	kubernetesNamespace = "k8s-namespace"
	kubernetesNodeName  = "k8s-node"
	kubernetesPodName   = "k8s-pod-name"
	kubernetesPodUID    = "k8s-pod-uid"

	uid       = "12321"
	mail      = "user@email.com"
	username  = "user"
	userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1"
	userIP    = "192.168.0.1"
)

var fullInput = map[string]interface{}{
	"service": map[string]interface{}{
		"name":        serviceName,
		"version":     serviceVersion,
		"environment": serviceEnvironment,
		"node": map[string]interface{}{
			"configured_name": serviceNodeName,
		},
		"language": map[string]interface{}{
			"name":    langName,
			"version": langVersion,
		},
		"runtime": map[string]interface{}{
			"name":    rtName,
			"version": rtVersion,
		},
		"framework": map[string]interface{}{
			"name":    fwName,
			"version": fwVersion,
		},
		"agent": map[string]interface{}{
			"name":    agentName,
			"version": agentVersion,
		},
	},
	"process": map[string]interface{}{
		"pid":   float64(pid),
		"ppid":  float64(ppid),
		"title": processTitle,
		"argv":  []interface{}{"apm-server"},
	},
	"system": map[string]interface{}{
		"detected_hostname":   detectedHostname,
		"configured_hostname": configuredHostname,
		"architecture":        systemArchitecture,
		"platform":            systemPlatform,
		"ip":                  systemIP,
		"container": map[string]interface{}{
			"id": containerID,
		},
		"kubernetes": map[string]interface{}{
			"namespace": kubernetesNamespace,
			"node": map[string]interface{}{
				"name": kubernetesNodeName,
			},
			"pod": map[string]interface{}{
				"name": kubernetesPodName,
				"uid":  kubernetesPodUID,
			},
		},
	},
	"user": map[string]interface{}{
		"id":         uid,
		"email":      mail,
		"username":   username,
		"ip":         userIP,
		"user-agent": userAgent,
	},
	"labels": map[string]interface{}{
		"k": "v", "n": 1, "f": 1.5, "b": false,
	},
}

func TestDecodeMetadata(t *testing.T) {
	output, err := DecodeMetadata(fullInput, false)
	require.NoError(t, err)
	assert.Equal(t, &metadata.Metadata{
		Service: metadata.Service{
			Name:        serviceName,
			Version:     serviceVersion,
			Environment: serviceEnvironment,
			Node:        metadata.ServiceNode{Name: serviceNodeName},
			Language:    metadata.Language{Name: langName, Version: langVersion},
			Runtime:     metadata.Runtime{Name: rtName, Version: rtVersion},
			Framework:   metadata.Framework{Name: fwName, Version: fwVersion},
			Agent:       metadata.Agent{Name: agentName, Version: agentVersion},
		},
		Process: metadata.Process{
			Pid:   pid,
			Ppid:  tests.IntPtr(ppid),
			Title: processTitle,
			Argv:  []string{"apm-server"},
		},
		System: metadata.System{
			DetectedHostname:   detectedHostname,
			ConfiguredHostname: configuredHostname,
			Architecture:       systemArchitecture,
			Platform:           systemPlatform,
			IP:                 net.ParseIP(systemIP),
			Container:          metadata.Container{ID: containerID},
			Kubernetes: metadata.Kubernetes{
				Namespace: kubernetesNamespace,
				NodeName:  kubernetesNodeName,
				PodName:   kubernetesPodName,
				PodUID:    kubernetesPodUID,
			},
		},
		User: metadata.User{
			Id:        uid,
			Email:     mail,
			Name:      username,
			IP:        net.ParseIP(userIP),
			UserAgent: userAgent,
		},
		Labels: common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
	}, output)
}

func BenchmarkDecodeMetadata(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeMetadata(fullInput, false); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMetadataRecycled(b *testing.B) {
	b.ReportAllocs()
	var meta metadata.Metadata
	for i := 0; i < b.N; i++ {
		if err := decodeMetadata(fullInput, false, metadataSchema, &meta); err != nil {
			b.Fatal(err)
		}
		for k := range meta.Labels {
			delete(meta.Labels, k)
		}
	}
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
