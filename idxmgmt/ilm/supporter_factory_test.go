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

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

func TestMakeDefaultSupporter(t *testing.T) {
	info := beat.Info{Beat: "mockapm", Version: "9.9.9"}

	t.Run("invalid config", func(t *testing.T) {
		cfg := common.MustNewConfigFrom(map[string]interface{}{
			"policy_name": "apm-test-span",
			"alias_name":  "test",
			"event":       123,
		})

		s, err := MakeDefaultSupporter(nil, info, cfg)
		assert.Nil(t, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy for 123 undefined")
	})

	t.Run("alias and policy missing in config", func(t *testing.T) {
		cfg := common.MustNewConfigFrom(map[string]interface{}{
			"event": 123,
		})

		s, err := MakeDefaultSupporter(nil, info, cfg)
		assert.Nil(t, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be configured")
	})

	t.Run("replace parameters in config", func(t *testing.T) {
		cfg := common.MustNewConfigFrom(map[string]interface{}{
			"policy_name": "%{[observer.name]}-%{[observer.version]}-%{[beat.name]}-%{[beat.version]}",
			"alias_name":  "alias-%{[observer.name]}-%{[observer.version]}-%{[beat.name]}-%{[beat.version]}",
			"event":       "span",
			"enabled":     "true",
		})

		s, err := MakeDefaultSupporter(nil, info, cfg)
		require.NoError(t, err)
		assert.Equal(t, "mockapm-9.9.9-mockapm-9.9.9", s.Policy().Name)
		assert.Equal(t, eventPolicies["span"], s.Policy().Body)
		assert.Equal(t, "alias-mockapm-9.9.9-mockapm-9.9.9", s.Alias().Name)
		assert.Equal(t, "000001", s.Alias().Pattern)
		assert.Equal(t, libilm.ModeEnabled, s.Mode())
	})

	t.Run("default mode", func(t *testing.T) {
		cfg := common.MustNewConfigFrom(map[string]interface{}{
			"policy_name": "policy",
			"alias_name":  "alias",
			"event":       "span",
		})

		s, err := MakeDefaultSupporter(nil, info, cfg)
		require.NoError(t, err)
		assert.Equal(t, libilm.ModeAuto, s.Mode())
	})

}
