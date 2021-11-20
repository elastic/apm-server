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
	"context"
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

	for name, host := range map[string]Host{
		"hostname":           {Hostname: detected},
		"ignored hostname":   {Name: configured},
		"full hostname info": {Hostname: detected, Name: configured},
		"full": {
			Hostname:     detected,
			Name:         configured,
			Architecture: "amd",
			Type:         "t2.medium",
			IP:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
			OS: OS{
				Platform: "osx",
				Full:     "Mac OS Mojave",
				Type:     "macos",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			event := &APMEvent{Host: host, Transaction: &Transaction{}}
			beatEvent := event.BeatEvent(context.Background())

			resultJSON, err := json.Marshal(beatEvent.Fields["host"])
			require.NoError(t, err)
			name := filepath.Join("test_approved", "host", strings.ReplaceAll(name, " ", "_"))
			approvaltest.ApproveJSON(t, name, resultJSON)
		})
	}
}
