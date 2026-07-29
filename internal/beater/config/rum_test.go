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

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp/logptest"

	"github.com/stretchr/testify/assert"
)

func TestRumSetup(t *testing.T) {
	rum := defaultRum()
	rum.SourceMapping.esOverrideConfigured = true
	rum.Enabled = true
	rum.SourceMapping.ESConfig = &elasticsearch.Config{APIKey: "id:apikey"}
	esCfg := config.MustNewConfigFrom(map[string]interface{}{
		"hosts": []interface{}{"cloud:9200"},
	})

	err := rum.setup(logptest.NewTestingLogger(t, "test"), esCfg)

	require.NoError(t, err)
	assert.Equal(t, elasticsearch.Hosts{"cloud:9200"}, rum.SourceMapping.ESConfig.Hosts)
	assert.Equal(t, "id:apikey", rum.SourceMapping.ESConfig.APIKey)
}

func TestDefaultRum(t *testing.T) {
	c := DefaultConfig()
	assert.Equal(t, defaultRum(), c.RumConfig)
}

func TestRumConfigMaxSourcemapSize(t *testing.T) {
	tests := []struct {
		name             string
		sourceMappingCfg map[string]interface{} // nil means use defaultRum()
		wantSize         uint64
		wantErr          error
	}{
		{
			name:     "default",
			wantSize: defaultMaxSourceMapSizeBytes,
		},
		{
			name:             "parse_mib_string",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": "5mib"},
			wantSize:         uint64(5 * 1024 * 1024),
		},
		{
			name:             "parse_mb_string",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": "5mb"},
			wantSize:         uint64(5 * 1000 * 1000),
		},
		{
			name:             "parse_integer",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": 2048},
			wantSize:         uint64(2048),
		},
		{
			name:             "parse_invalid_string",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": "invalid_size"},
			wantErr:          errParseSourceMapMaxSize,
		},
		{
			name:             "parse_zero_integer",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": 0},
			wantErr:          errParseSourceMapMaxSize,
		},
		{
			name:             "parse_zero_string",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": "0"},
			wantErr:          errParseSourceMapMaxSize,
		},
		{
			name:             "parse_negative_integer",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": -10},
			wantErr:          errParseSourceMapMaxSize,
		},
		{
			name:             "parse_empty_string",
			sourceMappingCfg: map[string]interface{}{"max_sourcemap_size": ""},
			wantErr:          errParseSourceMapMaxSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sourceMappingCfg == nil {
				c := defaultRum()
				assert.Equal(t, tt.wantSize, c.SourceMapping.MaxSourceMapSizeParsed)
				return
			}

			cfgMap := map[string]interface{}{
				"enabled":        true,
				"source_mapping": tt.sourceMappingCfg,
			}
			c, err := config.NewConfigFrom(cfgMap)
			require.NoError(t, err)
			var rum RumConfig
			err = c.Unpack(&rum)

			if tt.wantErr != nil {
				t.Logf("source map error: %s", err)
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantSize, rum.SourceMapping.MaxSourceMapSizeParsed)
			}
		})
	}
}
