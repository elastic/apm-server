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

package idxmgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

func TestMakeDefaultSupporter(t *testing.T) {
	info := beat.Info{}

	buildSupporter := func(c map[string]interface{}) (*supporter, error) {
		cfg, err := common.NewConfigFrom(c)
		require.NoError(t, err)
		s, err := MakeDefaultSupporter(nil, info, cfg)
		if s != nil {
			return s.(*supporter), err
		}
		return nil, err
	}

	t.Run("StdSupporter", func(t *testing.T) {
		s, err := buildSupporter(map[string]interface{}{
			"apm-server.ilm.enabled":       "auto",
			"setup.template.enabled":       "true",
			"output.elasticsearch.enabled": "true",
		})
		require.NoError(t, err)

		assert.True(t, s.Enabled())
		assert.NotNil(t, s.log)
		assert.True(t, s.templateConfig.Enabled)
		assert.True(t, s.ilmConfig.Enabled())
		assert.Equal(t, &esIndexConfig{}, s.esIdxCfg)
	})

	t.Run("ILMDisabled", func(t *testing.T) {
		s, err := buildSupporter(map[string]interface{}{
			"apm-server.ilm.enabled":       "false",
			"setup.template.enabled":       "true",
			"setup.template.name":          "custom",
			"setup.template.pattern":       "custom",
			"output.elasticsearch.index":   "custom-index",
			"output.elasticsearch.enabled": "true",
		})
		require.NoError(t, err)
		assert.False(t, s.ilmConfig.Enabled())
	})
	t.Run("SetupTemplateConfigConflicting", func(t *testing.T) {
		s, err := buildSupporter(map[string]interface{}{
			"output.elasticsearch.index": "custom-index",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "`setup.template.name` and `setup.template.pattern` have to be set ")
		assert.Nil(t, s)

	})
}
