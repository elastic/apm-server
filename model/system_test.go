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

	"github.com/elastic/apm-server/approvaltest"
)

func TestSystemTransformation(t *testing.T) {
	detected, configured := "host", "custom hostname"
	namespace := "staging"
	nodename, podname, podUID := "a.node", "a.pod", "b.podID"

	for name, system := range map[string]System{
		"hostname":           {DetectedHostname: detected},
		"ignored hostname":   {ConfiguredHostname: configured},
		"full hostname info": {DetectedHostname: detected, ConfiguredHostname: configured},
		"full": {
			DetectedHostname:   detected,
			ConfiguredHostname: configured,
			Architecture:       "amd",
			Platform:           "osx",
			FullPlatform:       "Mac OS Mojave",
			OSType:             "macos",
			Type:               "t2.medium",
			IP:                 net.ParseIP("127.0.0.1"),
			Container:          Container{ID: "1234"},
			Kubernetes:         Kubernetes{Namespace: namespace, NodeName: nodename, PodName: podname, PodUID: podUID},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var fields mapStr
			metadata := &Metadata{System: system}
			metadata.set(&fields, nil)
			resultJSON, err := json.Marshal(fields["host"])
			require.NoError(t, err)
			name := filepath.Join("test_approved", "system", strings.ReplaceAll(name, " ", "_"))
			approvaltest.ApproveJSON(t, name, resultJSON)
		})
	}
}

func TestNetworkTransformation(t *testing.T) {
	var fields mapStr
	metadata := &Metadata{System: System{Network: Network{ConnectionType: "3G",
		Carrier: Carrier{Name: "Three", MCC: "100", MNC: "200", ICC: "DK"}}}}
	metadata.set(&fields, nil)
	resultJSON, err := json.Marshal(fields["network"])
	require.NoError(t, err)
	name := filepath.Join("test_approved", "system", strings.ReplaceAll("network", " ", "_"))
	approvaltest.ApproveJSON(t, name, resultJSON)
}
