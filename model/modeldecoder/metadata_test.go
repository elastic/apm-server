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

	"github.com/elastic/apm-server/model"
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

	containerID         = "container-123"
	kubernetesNamespace = "k8s-namespace"
	kubernetesNodeName  = "k8s-node"
	kubernetesPodName   = "k8s-pod-name"
	kubernetesPodUID    = "k8s-pod-uid"

	uid       = "12321"
	mail      = "user@email.com"
	username  = "user"
	userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1"
)

var (
	systemIP = net.ParseIP("192.168.0.1")
	userIP   = net.ParseIP("192.168.0.1")
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
		"id":       uid,
		"email":    mail,
		"username": username,
	},
	"cloud": map[string]interface{}{
		"availability_zone": "australia-southeast1-a",
		"account": map[string]interface{}{
			"id":   "acct123",
			"name": "my-dev-account",
		},
		"instance": map[string]interface{}{
			"id":   "inst-foo123xyz",
			"name": "my-instance",
		},
		"machine": map[string]interface{}{
			"type": "n1-highcpu-96",
		},
		"project": map[string]interface{}{
			"id":   "snazzy-bobsled-123",
			"name": "Development",
		},
		"provider": "gcp",
		"region":   "australia-southeast1",
	},
	"labels": map[string]interface{}{
		"k": "v", "n": 1, "f": 1.5, "b": false,
	},
}

func metadata() *model.Metadata {
	return &model.Metadata{
		UserAgent: model.UserAgent{Original: userAgent},
		Client:    model.Client{IP: userIP},
		System:    model.System{IP: systemIP}}
}

func TestDecodeMetadata(t *testing.T) {
	output := metadata()
	require.NoError(t, DecodeMetadata(fullInput, false, output))
	assert.Equal(t, &model.Metadata{
		Service: model.Service{
			Name:        serviceName,
			Version:     serviceVersion,
			Environment: serviceEnvironment,
			Node:        model.ServiceNode{Name: serviceNodeName},
			Language:    model.Language{Name: langName, Version: langVersion},
			Runtime:     model.Runtime{Name: rtName, Version: rtVersion},
			Framework:   model.Framework{Name: fwName, Version: fwVersion},
			Agent:       model.Agent{Name: agentName, Version: agentVersion},
		},
		Process: model.Process{
			Pid:   pid,
			Ppid:  tests.IntPtr(ppid),
			Title: processTitle,
			Argv:  []string{"apm-server"},
		},
		System: model.System{
			DetectedHostname:   detectedHostname,
			ConfiguredHostname: configuredHostname,
			Architecture:       systemArchitecture,
			Platform:           systemPlatform,
			IP:                 systemIP,
			Container:          model.Container{ID: containerID},
			Kubernetes: model.Kubernetes{
				Namespace: kubernetesNamespace,
				NodeName:  kubernetesNodeName,
				PodName:   kubernetesPodName,
				PodUID:    kubernetesPodUID,
			},
		},
		User: model.User{
			ID:    uid,
			Email: mail,
			Name:  username,
		},
		UserAgent: model.UserAgent{
			Original: userAgent,
		},
		Client: model.Client{
			IP: userIP,
		},
		Cloud: model.Cloud{
			AccountID:        "acct123",
			AccountName:      "my-dev-account",
			AvailabilityZone: "australia-southeast1-a",
			InstanceID:       "inst-foo123xyz",
			InstanceName:     "my-instance",
			MachineType:      "n1-highcpu-96",
			ProjectID:        "snazzy-bobsled-123",
			ProjectName:      "Development",
			Provider:         "gcp",
			Region:           "australia-southeast1",
		},
		Labels: common.MapStr{"k": "v", "n": 1, "f": 1.5, "b": false},
	}, output)
}

func BenchmarkDecodeMetadata(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := DecodeMetadata(fullInput, false, &model.Metadata{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMetadataRecycled(b *testing.B) {
	b.ReportAllocs()
	var meta model.Metadata
	for i := 0; i < b.N; i++ {
		if err := DecodeMetadata(fullInput, false, &meta); err != nil {
			b.Fatal(err)
		}
		for k := range meta.Labels {
			delete(meta.Labels, k)
		}
	}
}

func TestDecodeMetadataInvalid(t *testing.T) {
	err := DecodeMetadata(nil, false, &model.Metadata{})
	require.EqualError(t, err, "failed to validate metadata: error validating JSON: input missing")

	err = DecodeMetadata("", false, &model.Metadata{})
	require.EqualError(t, err, "failed to validate metadata: error validating JSON: invalid input type")

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"service": map[string]interface{}{
			"agent": map[string]interface{}{},
			"name":  "name",
		},
	}
	require.NoError(t, DecodeMetadata(baseInput, false, &model.Metadata{}))

	for _, test := range []struct {
		input map[string]interface{}
		err   string
	}{
		{
			input: map[string]interface{}{"service": 123},
			err:   "service.*expected object, but got number",
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   "system.*expected object or null, but got number",
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   "process.*expected object or null, but got number",
		},
		{
			input: map[string]interface{}{"user": 123},
			err:   "user.*expected object or null, but got number",
		},
		{
			input: map[string]interface{}{"cloud": 123},
			err:   "cloud.*expected object or null, but got number",
		},
		{
			input: map[string]interface{}{"cloud": map[string]interface{}{}},
			err:   `cloud.*missing properties: "provider"`,
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
		err = DecodeMetadata(input, false, &model.Metadata{})
		require.Error(t, err)
		assert.Regexp(t, test.err, err.Error())
	}

}
