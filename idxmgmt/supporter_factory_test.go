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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/template"

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/idxmgmt/unmanaged"
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
		assert.Equal(t, "best_compression", s.templateConfig.Settings.Index["codec"])
		assert.Equal(t, libilm.ModeAuto, s.ilmConfig.Mode)
		assert.True(t, s.ilmConfig.Setup.Enabled)
		assert.Equal(t, unmanaged.Config{}, s.unmanagedIdxConfig)
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
		assert.Equal(t, libilm.ModeDisabled, s.ilmConfig.Mode)
		assert.True(t, s.ilmConfig.Setup.Enabled)
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

func TestMakeDefaultSupporterDataStreams(t *testing.T) {
	supporter, err := MakeDefaultSupporter(nil, beat.Info{}, common.MustNewConfigFrom(map[string]interface{}{
		"apm-server.data_streams.enabled": "true",
	}))
	require.NoError(t, err)

	// The data streams supporter does not set up templates or ILM. These
	// are expected to be set up externally, typically by Fleet.
	assert.False(t, supporter.Enabled())

	// Manager will fail when invoked; it should never be invoked automatically
	// as supporter.Enabled() returns false. It will be invoked when running the
	// "setup" command.
	manager := supporter.Manager(nil, nil)
	assert.NotNil(t, manager)
	ok, warnings := manager.VerifySetup(idxmgmt.LoadModeEnabled, idxmgmt.LoadModeEnabled)
	assert.True(t, ok)
	assert.Zero(t, warnings)
	err = manager.Setup(idxmgmt.LoadModeEnabled, idxmgmt.LoadModeEnabled)
	assert.EqualError(t, err, "index setup must be performed externally when using data streams, by installing the 'apm' integration package")

	selector, err := supporter.BuildSelector(nil)
	require.NoError(t, err)
	index, err := selector.Select(&beat.Event{
		Fields: common.MapStr{
			datastreams.TypeField:      datastreams.TracesType,
			datastreams.DatasetField:   "apm",
			datastreams.NamespaceField: "production",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "traces-apm-production", index)
}

func TestMakeDefaultSupporterDataStreamsWarnings(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	attrs := map[string]interface{}{
		"apm-server.data_streams.enabled":             "true",
		"apm-server.ilm.enabled":                      "auto",
		"apm-server.register.ingest.pipeline.enabled": "true",
		"output.elasticsearch.indices":                map[string]interface{}{},
		"setup.template.name":                         "custom",
		"setup.template.pattern":                      "custom",
	}

	s, err := MakeDefaultSupporter(logger, beat.Info{}, common.MustNewConfigFrom(attrs))
	assert.NoError(t, err)
	assert.NotNil(t, s)

	var warnings []string
	for _, record := range observed.All() {
		assert.Equal(t, zapcore.WarnLevel, record.Level, record.Message)
		warnings = append(warnings, record.Message)
	}
	assert.Equal(t, []string{
		"`setup.template` specified, but will be ignored as data streams are enabled",
		"`apm-server.ilm` specified, but will be ignored as data streams are enabled",
		"`apm-server.register.ingest.pipeline` specified, but will be ignored as data streams are enabled",
		"`output.elasticsearch.{index,indices}` specified, but will be ignored as data streams are enabled",
	}, warnings)
}

func TestNewIndexManagementConfig(t *testing.T) {
	cfg := common.MustNewConfigFrom(map[string]interface{}{
		"path.config":           "/dev/null",
		"setup.template.fields": "${path.config}/fields.yml",
	})
	indexManagementConfig, err := NewIndexManagementConfig(beat.Info{}, cfg)
	assert.NoError(t, err)
	require.NotNil(t, indexManagementConfig)

	templateConfig := template.DefaultConfig()
	templateConfig.Fields = "/dev/null/fields.yml"
	templateConfig.Settings = template.TemplateSettings{
		Index: map[string]interface{}{
			"codec": "best_compression",
			"mapping": map[string]interface{}{
				"total_fields": map[string]interface{}{
					"limit": uint64(2000),
				},
			},
			"number_of_shards": uint64(1),
		},
		Source: map[string]interface{}{
			"enabled": true,
		},
	}
	assert.Equal(t, templateConfig, indexManagementConfig.Template)
}
