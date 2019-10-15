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

package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
)

func TestIsRumEnabled(t *testing.T) {
	truthy := true
	for _, td := range []struct {
		c       *Config
		enabled bool
	}{
		{c: &Config{RumConfig: &RumConfig{Enabled: new(bool)}}, enabled: false},
		{c: &Config{RumConfig: &RumConfig{Enabled: &truthy}}, enabled: true},
	} {
		assert.Equal(t, td.enabled, td.c.RumConfig.IsEnabled())

	}
}

func TestDefaultRum(t *testing.T) {
	c := DefaultConfig("7.0.0")
	assert.Equal(t, defaultRum("7.0.0"), c.RumConfig)
}

func TestMemoizedSourcemapMapper(t *testing.T) {
	truthy := true
	esConfig, err := common.NewConfigFrom(map[string]interface{}{
		"hosts": []string{"localhost:0"},
	})
	require.NoError(t, err)
	mapping := SourceMapping{
		Cache:        &Cache{Expiration: 1 * time.Minute},
		IndexPattern: "apm-rum-test*",
		EsConfig:     esConfig,
	}

	for idx, td := range []struct {
		c      *Config
		mapper bool
		e      error
	}{
		{c: &Config{RumConfig: &RumConfig{}}, mapper: false, e: nil},
		{c: &Config{RumConfig: &RumConfig{Enabled: new(bool)}}, mapper: false, e: nil},
		{c: &Config{RumConfig: &RumConfig{Enabled: &truthy}}, mapper: false, e: nil},
		{c: &Config{RumConfig: &RumConfig{SourceMapping: &mapping}}, mapper: false, e: nil},
		{c: &Config{
			RumConfig: &RumConfig{
				Enabled: &truthy,
				SourceMapping: &SourceMapping{
					Cache:        &Cache{Expiration: 1 * time.Minute},
					IndexPattern: "apm-rum-test*",
				},
			}},
			mapper: false,
			e:      nil},
		{c: &Config{RumConfig: &RumConfig{Enabled: &truthy, SourceMapping: &mapping}},
			mapper: true,
			e:      nil},
	} {
		mapper, e := td.c.RumConfig.MemoizedSourcemapMapper()
		if td.mapper {
			assert.NotNil(t, mapper, fmt.Sprintf("Test number <%v> failed", idx))
		} else {
			assert.Nil(t, mapper, fmt.Sprintf("Test number <%v> failed", idx))
		}
		assert.Equal(t, td.e, e)
	}
}
