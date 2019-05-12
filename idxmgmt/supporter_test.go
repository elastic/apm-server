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
	"fmt"
	"testing"
	"time"

	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"

	"github.com/elastic/beats/libbeat/idxmgmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/idxmgmt/ilm"
)

func TestIndexSupport_Enabled(t *testing.T) {
	for name, test := range map[string]struct {
		expected bool
		cfg      common.MapStr
	}{
		"template default": {
			expected: true,
		},
		"template disabled": {
			expected: false,
			cfg:      common.MapStr{"setup.template.enabled": false},
		},
		"template enabled": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, defaultSupporter(t, test.cfg).Enabled())
		})
	}
}

func TestIndexSupport_ILM(t *testing.T) {
	noop, err := libilm.NoopSupport(nil, info, nil)
	require.NoError(t, err)
	ilmSupporter, err := ilm.MakeDefaultSupporter(nil, beat.Info{}, nil)
	require.NoError(t, err)
	assert.Equal(t, noop, ilmSupporter)
	assert.Equal(t, noop, defaultSupporter(t, nil).(*supporter).ilmSupporter)
}

func TestIndexSupport_BuildSelector(t *testing.T) {
	today := time.Now().UTC()
	day := today.Format("2006.01.02")
	testdata := map[string]struct {
		meta     common.MapStr
		expected string
		cfg      common.MapStr
		fields   common.MapStr
	}{
		"default index": {
			expected: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:   common.MapStr{},
		},
		"default onboarding": {
			expected: fmt.Sprintf("apm-7.0.0-onboarding-%s", day),
			fields:   common.MapStr{"processor.event": "onboarding"},
		},
		"default error": {
			expected: fmt.Sprintf("apm-7.0.0-error-%s", day),
			fields:   common.MapStr{"processor.event": "error"},
		},
		"default span": {
			expected: fmt.Sprintf("apm-7.0.0-span-%s", day),
			fields:   common.MapStr{"processor.event": "span"},
		},
		"default transaction": {
			expected: fmt.Sprintf("apm-7.0.0-transaction-%s", day),
			fields:   common.MapStr{"processor.event": "transaction"},
		},
		"default metric": {
			expected: fmt.Sprintf("apm-7.0.0-metric-%s", day),
			fields:   common.MapStr{"processor.event": "metric"},
		},
		"default sourcemap": {
			expected: "apm-7.0.0-sourcemap",
			fields:   common.MapStr{"processor.event": "sourcemap"},
		},
		"with meta information": {
			expected: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:   common.MapStr{"processor.event": "span"},
			meta:     common.MapStr{"index": "apm-7.0.0"},
		},
		"meta information overwrites config": {
			expected: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:   common.MapStr{"processor.event": "span"},
			meta:     common.MapStr{"index": "apm-7.0.0"},
			cfg:      common.MapStr{"index": "apm-customized"},
		},
		"custom index": {
			expected: "apm-customized",
			fields:   common.MapStr{"processor.event": "metric"},
			cfg:      common.MapStr{"index": "apm-customized"},
		},
		"custom indices, fallback to default index": {
			expected: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:   common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "error"}}}},
			},
		},
		"custom indices": {
			expected: "apm-custom-7.0.0-metric",
			fields:   common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}},
			},
		},
	}
	for name, test := range testdata {
		t.Run(name, func(t *testing.T) {
			fields := test.fields
			fields.Put("observer", common.MapStr{"version": "7.0.0"})
			testEvent := &beat.Event{
				Fields:    fields,
				Meta:      test.meta,
				Timestamp: today,
			}
			cfg, err := common.NewConfigFrom(test.cfg)
			require.NoError(t, err)

			s, err := defaultSupporter(t, nil).BuildSelector(cfg)
			assert.NoError(t, err)
			idx, err := s.Select(testEvent)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, idx)
		})
	}
}

var info = beat.Info{Beat: "testbeat", Version: "1.1.0"}

func defaultSupporter(t *testing.T, c common.MapStr) idxmgmt.Supporter {
	cfg, err := common.NewConfigFrom(c)
	require.NoError(t, err)
	supporter, err := MakeDefaultSupporter(nil, info, cfg)
	require.NoError(t, err)
	return supporter
}
