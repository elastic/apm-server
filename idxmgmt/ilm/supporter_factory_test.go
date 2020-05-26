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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
)

func TestMakeDefaultSupporter(t *testing.T) {
	info := beat.Info{Beat: "mockapm", Version: "9.9.9"}

	t.Run("invalid index name", func(t *testing.T) {
		cfg := Config{Setup: Setup{Mappings: Mappings{
			"error": Mapping{EventType: "error", PolicyName: defaultPolicyName,
				RolloverAlias: "%{[xyz.name]}-%{[observer.version]}-%{[beat.name]}-%{[beat.version]}"},
		}}}
		s, err := MakeDefaultSupporter(nil, info, 0, cfg)
		assert.Nil(t, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key not found")
	})

	t.Run("valid", func(t *testing.T) {
		cfg := Config{Setup: Setup{Policies: defaultPolicies(), Mappings: defaultMappings()}}

		s, err := MakeDefaultSupporter(nil, info, 0, cfg)
		require.NoError(t, err)
		assert.Equal(t, 5, len(s))
		var aliases []string
		for _, sup := range s {
			aliases = append(aliases, sup.Alias().Name)
			assert.Equal(t, defaultPolicyName, sup.Policy().Name)
		}
		defaultAliases := []string{"apm-9.9.9-error", "apm-9.9.9-span", "apm-9.9.9-transaction", "apm-9.9.9-metric", "apm-9.9.9-profile"}
		assert.ElementsMatch(t, defaultAliases, aliases)
	})
}
