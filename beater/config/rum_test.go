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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/stretchr/testify/assert"
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

func TestRumSetup(t *testing.T) {
	rum := defaultRum()
	rum.SourceMapping.esConfigured = true
	rum.Enabled = &rum.SourceMapping.esConfigured
	rum.SourceMapping.ESConfig = &elasticsearch.Config{APIKey: "id:apikey"}
	esCfg := common.MustNewConfigFrom(map[string]interface{}{
		"hosts": []interface{}{"cloud:9200"},
	})

	err := rum.setup(logp.NewLogger("test"), true, esCfg)

	require.NoError(t, err)
	assert.Equal(t, elasticsearch.Hosts{"cloud:9200"}, rum.SourceMapping.ESConfig.Hosts)
	assert.Equal(t, "id:apikey", rum.SourceMapping.ESConfig.APIKey)
}

func TestDefaultRum(t *testing.T) {
	c := DefaultConfig()
	assert.Equal(t, defaultRum(), c.RumConfig)
}

func TestSourceMapMaxLineLength(t *testing.T) {
	maxLen := uint(512)
	srcMapping := defaultSourcemapping()
	srcMapping.MaxLineLength = maxLen

	parsedSrcMapping := new(SourceMapping)

	parsedSrcMapping.Unpack(common.MustNewConfigFrom(map[string]interface{}{
		"max_line_length": maxLen,
	}))

	assert.Equal(t, srcMapping.MaxLineLength, parsedSrcMapping.MaxLineLength)
}
