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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
)

var (
	info  = beat.Info{Beat: "testbeat", Version: "7.0.0"}
	today = time.Now().UTC()
	day   = today.Format("2006.01.02")
)

func defaultSupporter(t *testing.T, m common.MapStr) *supporter {
	c := common.MapStr{
		"output.elasticsearch.enabled": true,
		"setup.template.name":          "custom",
		"setup.template.pattern":       "custom*",
	}
	c.DeepUpdate(m)

	cfg, err := common.NewConfigFrom(c)
	require.NoError(t, err)
	sup, err := MakeDefaultSupporter(nil, info, cfg)
	require.NoError(t, err)
	return sup.(*supporter)
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
			cfg:      common.MapStr{"setup.template.enabled": true, "apm-server.ilm.setup.enabled": false},
		},
		"ilm enabled": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.setup.enabled": false},
		},
		"ilm managed": {
			expected: true,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.setup.enabled": true},
		},
		"nothing enabled": {
			expected: false,
			cfg:      common.MapStr{"setup.template.enabled": false, "apm-server.ilm.setup.enabled": false, "apm-server.ilm.enabled": false},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, defaultSupporter(t, test.cfg).Enabled())
		})
	}
}

func TestIndexSupport_BuildSelector(t *testing.T) {

	type testdata struct {
		meta     common.MapStr
		noIlm    string
		withIlm  string
		ilmAuto  string
		expected string
		cfg      common.MapStr
		fields   common.MapStr
		alias    bool
	}

	cases := map[string]testdata{
		"DefaultIndex": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:  common.MapStr{},
		},
		"DefaultOnboarding": {
			noIlm:   fmt.Sprintf("apm-7.0.0-onboarding-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-onboarding-%s", day),
			fields:  common.MapStr{"processor.event": "onboarding"},
		},
		"DefaultError": {
			noIlm:   fmt.Sprintf("apm-7.0.0-error-%s", day),
			withIlm: "apm-7.0.0-error",
			fields:  common.MapStr{"processor.event": "error"},
		},
		"DefaultSpan": {
			noIlm:   fmt.Sprintf("apm-7.0.0-span-%s", day),
			withIlm: "apm-7.0.0-span",
			fields:  common.MapStr{"processor.event": "span"},
		},
		"DefaultTransaction": {
			noIlm:   fmt.Sprintf("apm-7.0.0-transaction-%s", day),
			withIlm: "apm-7.0.0-transaction",
			fields:  common.MapStr{"processor.event": "transaction"},
		},
		"DefaultMetric": {
			noIlm:   fmt.Sprintf("apm-7.0.0-metric-%s", day),
			withIlm: "apm-7.0.0-metric",
			fields:  common.MapStr{"processor.event": "metric"},
		},
		"DefaultSourcemap": {
			noIlm:   "apm-7.0.0-sourcemap",
			withIlm: "apm-7.0.0-sourcemap",
			fields:  common.MapStr{"processor.event": "sourcemap"},
		},
		"MetaInformationAlias": {
			noIlm:   "apm-7.0.0-meta",
			withIlm: "apm-7.0.0-meta", //meta alias overwrite ilm
			ilmAuto: "apm-7.0.0-meta",
			fields:  common.MapStr{"processor.event": "span"},
			meta:    common.MapStr{"alias": "APM-7.0.0-meta", "index": "test-123"},
			cfg:     common.MapStr{"output.elasticsearch.index": "apm-customized"},
		},
		"MetaInformationIndex": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: fmt.Sprintf("apm-7.0.0-%s", day), //meta index overwrites ilm
			ilmAuto: fmt.Sprintf("apm-7.0.0-%s", day),
			fields:  common.MapStr{"processor.event": "span"},
			meta:    common.MapStr{"index": "APM-7.0.0"},
			cfg:     common.MapStr{"output.elasticsearch.index": "apm-customized"},
		},
		"CustomIndex-lowercased": {
			noIlm:   "apm-customized",
			withIlm: "apm-7.0.0-metric", //custom index ignored when ilm enabled
			ilmAuto: "apm-customized",   //custom respected for ilm auto
			fields:  common.MapStr{"processor.event": "metric"},
			cfg:     common.MapStr{"output.elasticsearch.index": "APM-customized"},
		},
		"DifferentCustomIndices": {
			noIlm:   fmt.Sprintf("apm-7.0.0-%s", day),
			withIlm: "apm-7.0.0-metric",               //custom index ignored when ilm enabled
			ilmAuto: fmt.Sprintf("apm-7.0.0-%s", day), //custom respected for ilm auto
			fields:  common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"output.elasticsearch.indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "error"}}}},
			},
		},
		"CustomIndices": {
			noIlm:   "apm-custom-7.0.0-metric",
			withIlm: "apm-7.0.0-metric",        //custom index ignored when ilm enabled
			ilmAuto: "apm-custom-7.0.0-metric", //custom respected for ilm auto
			fields:  common.MapStr{"processor.event": "metric"},
			cfg: common.MapStr{
				"output.elasticsearch.indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}},
			},
		},
	}

	ilmSupportedHandler := newMockClientHandler("7.2.0")
	ilmUnsupportedHandler := newMockClientHandler("6.2.0")

	checkIndexSelector := func(t *testing.T, name string, test testdata, handler libidxmgmt.ClientHandler) {
		t.Run(name, func(t *testing.T) {
			// create initialized supporter and selector

			supporter := defaultSupporter(t, test.cfg)
			err := supporter.Manager(handler, nil).Setup(libidxmgmt.LoadModeDisabled, libidxmgmt.LoadModeDisabled)
			require.NoError(t, err)

			s, err := supporter.BuildSelector(nil)
			require.NoError(t, err)

			// test selected index
			idx, err := s.Select(testEvent(test.fields, test.meta))
			require.NoError(t, err)
			assert.Equal(t, test.expected, idx)

			// test if alias
			assert.Equal(t, test.alias, s.IsAlias())
		})
	}

	for name, test := range cases {
		if test.cfg == nil {
			test.cfg = common.MapStr{}
		}

		//ilm true and supported
		test.cfg["apm-server.ilm.enabled"] = true
		test.expected = test.withIlm
		test.alias = true
		checkIndexSelector(t, "ILMTrueSupported"+name, test, ilmSupportedHandler)

		//ilm=false
		test.cfg["apm-server.ilm.enabled"] = false
		test.expected = test.noIlm
		test.alias = false
		checkIndexSelector(t, "ILMFalse"+name, test, ilmSupportedHandler)

		//ilm=auto and supported
		test.cfg["apm-server.ilm.enabled"] = "auto"
		test.expected = test.withIlm
		test.alias = true
		if test.ilmAuto != "" {
			test.expected = test.ilmAuto
			test.alias = false
		}
		checkIndexSelector(t, "ILMAutoSupported"+name, test, ilmSupportedHandler)

		//ilm auto but unsupported
		test.cfg["apm-server.ilm.enabled"] = "auto"
		test.expected = test.noIlm
		test.alias = false
		checkIndexSelector(t, "ILMAutoUnsupported"+name, test, ilmUnsupportedHandler)
	}
}

func testEvent(fields, meta common.MapStr) *beat.Event {
	// needs to be set for indices with unmanaged lifecycle (not ILM)
	// as the version replacement happens on every event and not on initialization
	fields.Put("observer", common.MapStr{"version": "7.0.0"})
	return &beat.Event{
		Fields:    fields,
		Meta:      meta,
		Timestamp: today,
	}
}
