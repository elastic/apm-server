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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/processors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudEnv(t *testing.T) {
	defer os.Unsetenv(cloudEnv)

	// no cloud environment variable set
	settings := DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	assert.Equal(t, common.NewConfig(), settings.ConfigOverrides[1].Config)

	assertEqual := func(t *testing.T, cfg *common.Config, key string, expected float64) {
		val, err := cfg.Float(key, -1)
		require.NoError(t, err)
		assert.Equal(t, expected, val)
	}

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

func TestProcessorsDisallowed(t *testing.T) {
	logger := logp.NewLogger("")
	settings := DefaultSettings()

	// If no "processors" config is specified, the supporter.Create method should return nil.
	supporter, err := settings.Processing(beat.Info{}, logger, common.MustNewConfigFrom(map[string]interface{}{}))
	assert.NoError(t, err)
	assert.NotNil(t, supporter)
	processor, err := supporter.Create(beat.ProcessingConfig{}, false)
	assert.NoError(t, err)
	assert.Nil(t, processor)

	supporter, err = settings.Processing(beat.Info{}, logger, common.MustNewConfigFrom(map[string]interface{}{
		"processors": []interface{}{
			map[string]interface{}{
				"add_cloud_metadata": map[string]interface{}{},
			},
		},
	}))
	assert.EqualError(t, err, "libbeat processors are not supported")
	assert.Nil(t, supporter)
}

func TestProcessorsFromConfigNotNil(t *testing.T) {
	logger := logp.NewLogger("")
	settings := DefaultSettings()

	supporter, err := settings.Processing(beat.Info{}, logger, common.MustNewConfigFrom(map[string]interface{}{}))
	require.NoError(t, err)
	assert.NotNil(t, supporter)

	processors, err := processors.New(processors.PluginConfig{
		common.MustNewConfigFrom(map[string]interface{}{
			"drop_event": map[string]interface{}{
				"when": map[string]interface{}{
					"contains": map[string]interface{}{
						"processor.event": "log",
					},
				},
			},
		}),
	})
	require.NoError(t, err)

	// The supporter is used by the pipeline client to add any more processors.
	// We're asserting that the processors are returned and not nil.
	processor, err := supporter.Create(beat.ProcessingConfig{Processor: processors}, false)
	assert.NoError(t, err)
	assert.NotNil(t, processor)
}
