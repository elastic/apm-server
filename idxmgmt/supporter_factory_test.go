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
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/datastreams"
)

func TestNewSupporter(t *testing.T) {
	supporter, err := NewSupporter(nil, beat.Info{}, common.MustNewConfigFrom(map[string]interface{}{}))
	require.NoError(t, err)
	require.NotNil(t, supporter)

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

func TestNewSupporterWarnings(t *testing.T) {
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

	NewSupporter(logger, beat.Info{}, common.MustNewConfigFrom(attrs))

	var warnings []string
	for _, record := range observed.All() {
		assert.Equal(t, zapcore.WarnLevel, record.Level, record.Message)
		warnings = append(warnings, record.Message)
	}
	assert.Equal(t, []string{
		"`apm-server.data_streams` specified, but was removed in 8.0 and will be ignored: data streams are always enabled",
		"`apm-server.ilm` specified, but was removed in 8.0 and will be ignored: ILM policies are managed by Fleet",
		"`apm-server.register.ingest.pipeline` specified, but was removed in 8.0 and will be ignored: ingest pipelines are managed by Fleet",
		"`output.elasticsearch.indices` specified, but was removed in 8.0 and will be ignored: indices cannot be customised, APM Server now produces data streams",
		"`setup.template` specified, but was removed in 8.0 and will be ignored: index templates are managed by Fleet",
	}, warnings)
}
