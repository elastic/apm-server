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
	"testing"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/processors"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessorsDisallowed(t *testing.T) {
	logger := logp.NewLogger("")
	settings := DefaultSettings()

	// If no "processors" config is specified, the supporter.Create method should return nil.
	supporter, err := settings.Processing(beat.Info{}, logger, agentconfig.MustNewConfigFrom(map[string]interface{}{}))
	assert.NoError(t, err)
	assert.NotNil(t, supporter)
	processor, err := supporter.Create(beat.ProcessingConfig{}, false)
	assert.NoError(t, err)
	assert.Nil(t, processor)

	supporter, err = settings.Processing(beat.Info{}, logger, agentconfig.MustNewConfigFrom(map[string]interface{}{
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

	supporter, err := settings.Processing(beat.Info{}, logger, agentconfig.MustNewConfigFrom(map[string]interface{}{}))
	require.NoError(t, err)
	assert.NotNil(t, supporter)

	processors, err := processors.New(processors.PluginConfig{
		agentconfig.MustNewConfigFrom(map[string]interface{}{
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
