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
	var cloudMatrix = map[string]struct {
		worker      int
		bulkMaxSize int
		events      int
		minEvents   int
	}{
		"512":   {5, 267, 2000, 267},
		"1024":  {7, 381, 4000, 381},
		"2048":  {10, 533, 8000, 533},
		"4096":  {14, 762, 16000, 762},
		"8192":  {20, 1067, 32000, 1067},
		"16384": {20, 2133, 64000, 2133},
		"32768": {20, 4267, 128000, 4267},
	}
	for capacity, throughputSettings := range cloudMatrix {
		os.Setenv(cloudEnv, capacity)
		settings = DefaultSettings()
		assert.Len(t, settings.ConfigOverrides, 2)
		cfg := settings.ConfigOverrides[1].Config
		assert.NotNil(t, cfg)
		assertEqual(t, cfg, "output.elasticsearch.worker", float64(throughputSettings.worker))
		assertEqual(t, cfg, "output.elasticsearch.bulk_max_size", float64(throughputSettings.bulkMaxSize))
		assertEqual(t, cfg, "queue.mem.events", float64(throughputSettings.events))
		assertEqual(t, cfg, "queue.mem.flush.min_events", float64(throughputSettings.minEvents))
		assertEqual(t, cfg, "output.elasticsearch.compression_level", 5)
	}
}

<<<<<<< HEAD
func assertEqual(t *testing.T, cfg *common.Config, key string, expected float64) {
	val, err := cfg.Float(key, -1)
	require.NoError(t, err)
	assert.Equal(t, expected, val)
=======
	compression, err := cfg.Int("output.elasticsearch.compression_level", -1)
	require.NoError(t, err)
	assert.Equal(t, int64(5), compression)

	// bad cloud environment value
	os.Setenv(cloudEnv, "123")
	settings = DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	assert.Equal(t, common.NewConfig(), settings.ConfigOverrides[1].Config)
>>>>>>> 1c09eed7 ([cloud] default to medium compression (#5446))
}
