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

package cmd

import (
	"os"
	"testing"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudEnv(t *testing.T) {
	defer os.Unsetenv(cloudEnv)

	// no cloud environment variable set
	settings := DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	assert.Equal(t, common.NewConfig(), settings.ConfigOverrides[1].Config)

	// cloud environment picked up
	os.Setenv(cloudEnv, "512")
	settings = DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	cfg := settings.ConfigOverrides[1].Config
	assert.NotNil(t, cfg)
	workers, err := cfg.Int("output.elasticsearch.worker", -1)
	require.NoError(t, err)
	assert.Equal(t, int64(5), workers)

	compression, err := cfg.Int("output.elasticsearch.compression_level", -1)
	require.NoError(t, err)
	assert.Equal(t, int64(5), compression)

	// bad cloud environment value
	os.Setenv(cloudEnv, "123")
	settings = DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	assert.Equal(t, common.NewConfig(), settings.ConfigOverrides[1].Config)
}
