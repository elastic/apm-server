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

package model

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/approvaltest"
)

func TestSystemTransformation(t *testing.T) {
	detected, configured := "host", "custom hostname"
	namespace := "staging"
	nodename, podname, podUID := "a.node", "a.pod", "b.podID"

	for name, system := range map[string]System{
		"hostname":                               System{DetectedHostname: detected},
		"ignored hostname":                       System{ConfiguredHostname: configured},
		"full hostname info":                     System{DetectedHostname: detected, ConfiguredHostname: configured},
		"k8s nodename with hostname":             System{Kubernetes: Kubernetes{NodeName: nodename}, DetectedHostname: detected},
		"k8s nodename with configured hostname":  System{Kubernetes: Kubernetes{NodeName: nodename}, ConfiguredHostname: configured},
		"k8s podname":                            System{Kubernetes: Kubernetes{PodName: podname}, DetectedHostname: detected},
		"k8s podUID":                             System{Kubernetes: Kubernetes{PodUID: podUID}, DetectedHostname: detected},
		"k8s namespace":                          System{Kubernetes: Kubernetes{Namespace: namespace}, DetectedHostname: detected},
		"k8s podname with configured hostname":   System{Kubernetes: Kubernetes{PodName: podname}, DetectedHostname: detected, ConfiguredHostname: configured},
		"k8s podUID with configured hostname":    System{Kubernetes: Kubernetes{PodUID: podUID}, DetectedHostname: detected, ConfiguredHostname: configured},
		"k8s namespace with configured hostname": System{Kubernetes: Kubernetes{Namespace: namespace}, DetectedHostname: detected, ConfiguredHostname: configured},
		"k8s empty":                              System{Kubernetes: Kubernetes{}, DetectedHostname: detected, ConfiguredHostname: configured},
		"full": System{
			DetectedHostname:   detected,
			ConfiguredHostname: configured,
			Architecture:       "amd",
			Platform:           "osx",
			IP:                 net.ParseIP("127.0.0.1"),
			Container:          Container{ID: "1234"},
			Kubernetes:         Kubernetes{Namespace: namespace, NodeName: nodename, PodName: podname, PodUID: podUID},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fields := make(common.MapStr)
			metadata := &Metadata{System: system}
			metadata.Set(fields, nil)
			resultJSON, err := json.Marshal(fields["host"])
			require.NoError(t, err)
			name := filepath.Join("test_approved", "system", strings.ReplaceAll(name, " ", "_"))
			approvaltest.ApproveJSON(t, name, resultJSON)
		})
	}
}
