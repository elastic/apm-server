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

package ilm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	libcommon "github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/idxmgmt/common"
)

func TestMakeDefaultSupporter(t *testing.T) {
	info := beat.Info{Beat: "mockapm", Version: "9.9.9"}
	input := `{"setup":{"require_policy":false,"mapping":[{"event_type":"span","policy_name":"rollover-10d"}]}}`
	cfg, err := NewConfig(info, libcommon.MustNewConfigFrom(input))
	require.NoError(t, err)

	s, err := MakeDefaultSupporter(nil, 0, cfg)
	require.NoError(t, err)
	assert.Equal(t, 5, len(s))
	var aliases []string
	for _, sup := range s {
		aliases = append(aliases, sup.Alias().Name)
		expectedPolicyName := defaultPolicyName
		if strings.Contains(sup.Alias().Name, "span") {
			expectedPolicyName = "rollover-10d"
		}
		assert.Equal(t, expectedPolicyName, sup.Policy().Name)
	}
	var defaultAliases []string
	for _, et := range common.EventTypes {
		defaultAliases = append(defaultAliases, "apm-9.9.9-"+et)
	}
	assert.ElementsMatch(t, defaultAliases, aliases)
}
