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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
)

var info = beat.Info{Beat: "testbeat", Version: "1.1.0"}

func defaultSupporter(t *testing.T, c common.MapStr) libidxmgmt.Supporter {
	cfg, err := common.NewConfigFrom(c)
	require.NoError(t, err)
	supporter, err := MakeDefaultSupporter(nil, info, cfg)
	require.NoError(t, err)
	return supporter
}

func TestIndexSupport_Enabled(t *testing.T) {
	for name, test := range map[string]struct {
		expected bool
		cfg      common.MapStr
	}{
		"template default": {
			expected: true,
		},
		"template enabled": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": true},
		},
		"template and ilm disabled": {
			expected: false,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
		},
		"ilm enabled": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": true},
		},
		"ilm auto": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": "auto"},
		},
		"ilm default": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": false},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, defaultSupporter(t, test.cfg).Enabled())
		})
	}
}
func TestIndexSupport_BuildSelector(t *testing.T) {
	today := time.Now().UTC()
	day := today.Format("2006.01.02")

	type testdata struct {
		meta     common.MapStr
		noIlm    string
		withIlm  string
		expected string
		cfg      common.MapStr
		fields   common.MapStr
	}

	cases := map[string]testdata{
		"default index": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:  common.MapStr{},
		},
		"default onboarding": {
			noIlm:   fmt.Sprintf("apm-7.0.0-onboarding-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-onboarding-%s", day),
			fields:  common.MapStr{"processor.event": "onboarding"},
		},
		"default error": {
			noIlm:   fmt.Sprintf("apm-7.0.0-error-%s", day),
			withIlm: "apm-7.0.0-error",
			fields:  common.MapStr{"processor.event": "error"},
		},
		"default span": {
			noIlm:   fmt.Sprintf("apm-7.0.0-span-%s", day),
			withIlm: "apm-7.0.0-span",
			fields:  common.MapStr{"processor.event": "span"},
		},
		"default transaction": {
			noIlm:   fmt.Sprintf("apm-7.0.0-transaction-%s", day),
			withIlm: "apm-7.0.0-transaction",
			fields:  common.MapStr{"processor.event": "transaction"},
		},
		"default metric": {
			noIlm:   fmt.Sprintf("apm-7.0.0-metric-%s", day),
			withIlm: "apm-7.0.0-metric",
			fields:  common.MapStr{"processor.event": "metric"},
		},
		"default sourcemap": {
			noIlm:   "apm-7.0.0-sourcemap",
			withIlm: "apm-7.0.0-sourcemap",
			fields:  common.MapStr{"processor.event": "sourcemap"},
		},
		"meta alias information overwrites config": {
			noIlm:   "apm-7.0.0-meta",
			withIlm: "apm-7.0.0-meta", //meta overwrites ilm
			fields:  common.MapStr{"processor.event": "span"},
			meta:    common.MapStr{"alias": "apm-7.0.0-meta", "index": "test-123"},
			cfg:     common.MapStr{"index": "apm-customized"},
		},
		"meta information overwrites config": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-%s", day), //meta overwrites ilm
			fields:  common.MapStr{"processor.event": "span"},
			meta:    common.MapStr{"index": "apm-7.0.0"},
			cfg:     common.MapStr{"index": "apm-customized"},
		},
		"custom index": {
			noIlm:   "apm-customized",
			withIlm: "apm-7.0.0-metric", //custom index ignored when ilm enabled
			fields:  common.MapStr{"processor.event": "metric"},
			cfg:     common.MapStr{"index": "apm-customized"},
		},
		"custom indices, fallback to default index": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: "apm-7.0.0-metric", //custom index ignored when ilm enabled
			fields:  common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "error"}}}},
			},
		},
		"custom indices": {
			noIlm:   "apm-custom-7.0.0-metric",
			withIlm: "apm-7.0.0-metric", //custom index ignored when ilm enabled
			fields:  common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}},
			},
		},
	}

	checkIndexSelector := func(t *testing.T, name string, test testdata) {
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

			s, err := defaultSupporter(t, test.cfg).BuildSelector(cfg)
			assert.NoError(t, err)
			idx, err := s.Select(testEvent)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, idx)
		})
	}

	for name, test := range cases {
		//ilm=true
		test.expected = test.withIlm
		checkIndexSelector(t, name, test)

		//ilm=false
		test.expected = test.noIlm
		if test.cfg == nil {
			test.cfg = common.MapStr{}
		}
		test.cfg["apm-server.ilm.enabled"] = false
		checkIndexSelector(t, name, test)
	}
}
