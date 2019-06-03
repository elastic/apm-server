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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/template"
)

type mockClientHandler struct {
	aliases, policies []string

	tmplLoadCt, tmplLoadWithILMCt int
	tmplOrderILMCt                int
	tmplForce                     bool

	ilmSupported bool
}

var (
	info          = beat.Info{Beat: "testbeat", Version: "1.1.0"}
	ilmEnabledKey = "apm-server.ilm.enabled"
)

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
		//ilm=false
		test.expected = test.noIlm
		checkIndexSelector(t, name, test)

		//ilm=true
		test.expected = test.withIlm
		if test.cfg == nil {
			test.cfg = common.MapStr{}
		}
		test.cfg[ilmEnabledKey] = true
		checkIndexSelector(t, name, test)
	}
}

func TestIndexManager_VerifySetup(t *testing.T) {
	for name, setup := range map[string]struct {
		tmpl, ilm         bool
		loadTmpl, loadILM libidxmgmt.LoadMode
		ok                bool
		warn              string
	}{
		"load template with ilm without loading ilm": {
			ilm: true, tmpl: true,
			loadTmpl: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeDisabled,
			warn: "whithout loading ILM policy and alias",
		},
		"load ilm without template": {
			ilm: true, loadILM: libidxmgmt.LoadModeEnabled,
			warn: "without loading template is not recommended",
		},
		"template disabled but loading enabled": {
			loadTmpl: libidxmgmt.LoadModeEnabled,
			warn:     "loading not enabled",
		},
		"ilm disabled but loading enabled": {
			loadILM: libidxmgmt.LoadModeEnabled, tmpl: true,
			warn: "loading not enabled",
		},
		"ilm enabled but loading disabled": {
			ilm: true, loadILM: libidxmgmt.LoadModeDisabled,
			warn: "loading not enabled",
		},
		"template enabled but loading disabled": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeDisabled,
			warn: "loading not enabled",
		},
		"everything enabled": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled, ilm: true, loadILM: libidxmgmt.LoadModeEnabled,
			ok: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := common.NewConfigFrom(common.MapStr{
				"apm-server.ilm.enabled": setup.ilm,
				"setup.template.enabled": setup.tmpl,
			})
			require.NoError(t, err)
			support, err := MakeDefaultSupporter(nil, beat.Info{}, cfg)
			require.NoError(t, err)
			manager := support.Manager(newMockClientHandler(true), nil)
			ok, warn := manager.VerifySetup(setup.loadTmpl, setup.loadILM)
			assert.Equal(t, setup.ok, ok)
			assert.Contains(t, warn, setup.warn)
		})
	}
}

func TestManager_Setup(t *testing.T) {
	fields := []byte("apm-server fields")
	errIlmNotSupported := "ILM not supported"
	for name, test := range map[string]struct {
		cfg                       common.MapStr
		tLoad, ilmLoad            libidxmgmt.LoadMode
		tCt, tWithILMCt, pCt, aCt int
		ilmSupported              bool
		errMsg                    string
	}{
		"default load template without ilm": {
			ilmSupported: true,
			cfg:          nil,
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5,
		},
		"load template without ilm": {
			ilmSupported: true,
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5,
		},
		"no dependency on ilm support": {
			ilmSupported: false,
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5,
		},
		"load template with ilm settings": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeDisabled,
			tCt: 5, tWithILMCt: 4,
		},
		"load template with ilm settings when ilm is not supported": {
			ilmSupported: false,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeDisabled,
			errMsg: errIlmNotSupported,
			tCt:    0, tWithILMCt: 0,
		},
		"load template disabled": {
			ilmSupported: true,
			cfg:          nil,
			tLoad:        libidxmgmt.LoadModeDisabled,
		},
		"template disabled": {
			ilmSupported: true,
			cfg:          common.MapStr{"setup.template.enabled": false},
			tLoad:        libidxmgmt.LoadModeEnabled,
		},
		"force load template": {
			ilmSupported: true,
			cfg:          nil,
			tLoad:        libidxmgmt.LoadModeForce,
			tCt:          5,
		},
		"template disabled ilm enabled loading disabled": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true, "setup.template.enabled": false},
			ilmLoad:      libidxmgmt.LoadModeUnset,
		},
		"do not load template load ilm": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true, "setup.template.enabled": false},
			ilmLoad:      libidxmgmt.LoadModeEnabled,
			pCt:          3, aCt: 3,
		},
		"try loading ilm when ilm not supported": {
			ilmSupported: false,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true, "setup.template.enabled": false},
			ilmLoad:      libidxmgmt.LoadModeEnabled,
			errMsg:       errIlmNotSupported,
			tCt:          0, tWithILMCt: 0,
		},
		"do not load template force load ilm": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeDisabled, ilmLoad: libidxmgmt.LoadModeForce,
			pCt: 4, aCt: 3,
		},
		"load template and ilm": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 3, aCt: 3,
		},
		"try loading template and ilm when ilm is not supported": {
			ilmSupported: false,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			errMsg: errIlmNotSupported,
			tCt:    0, tWithILMCt: 0,
		},
		"force load template force load ilm": {
			ilmSupported: true,
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:        libidxmgmt.LoadModeForce, ilmLoad: libidxmgmt.LoadModeForce,
			tCt: 5, tWithILMCt: 4, pCt: 4, aCt: 3,
		},
	} {

		t.Run(name, func(t *testing.T) {
			ch := newMockClientHandler(test.ilmSupported)
			m := defaultSupporter(t, test.cfg).Manager(ch, libidxmgmt.BeatsAssets(fields))
			idxManager := m.(*manager)
			err := idxManager.Setup(test.tLoad, test.ilmLoad)
			if test.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)
			}

			assert.Equal(t, test.pCt, len(ch.policies))
			assert.Equal(t, test.aCt, len(ch.aliases))
			require.Equal(t, test.tCt, ch.tmplLoadCt)
			assert.Equal(t, test.tWithILMCt, ch.tmplLoadWithILMCt)
			if test.tCt > 0 {
				assert.Equal(t, 1, ch.tmplLoadCt-ch.tmplOrderILMCt)
				assert.Equal(t, test.tCt-1, ch.tmplOrderILMCt)
			}
		})
	}

}

var existingILM = fmt.Sprintf("apm-%s-transaction", info.Version)

func newMockClientHandler(ilmSupported bool) *mockClientHandler {
	return &mockClientHandler{ilmSupported: ilmSupported}
}

func (h *mockClientHandler) Load(config template.TemplateConfig, _ beat.Info, fields []byte, migration bool) error {
	h.tmplLoadCt++
	if config.Settings.Index != nil && config.Settings.Index["lifecycle.name"] != nil {
		h.tmplLoadWithILMCt++
		if len(fields) > 0 {
			return errors.New("fields should be empty")
		}
	}
	if config.Order == 2 {
		h.tmplOrderILMCt++
	}
	if config.Order != 1 && config.Order != 2 {
		return errors.New("unexpected template order")
	}
	h.tmplForce = config.Overwrite
	return nil
}

func (h *mockClientHandler) CheckILMEnabled(m libilm.Mode) (bool, error) {
	enabled := m == libilm.ModeEnabled
	if h.ilmSupported {
		return enabled, nil
	}
	return enabled, errors.New("ILM not supported")
}

func (h *mockClientHandler) HasAlias(name string) (bool, error) {
	return name == existingILM, nil
}

func (h *mockClientHandler) CreateAlias(alias libilm.Alias) error {
	h.aliases = append(h.aliases, alias.Name)
	return nil
}

func (h *mockClientHandler) HasILMPolicy(name string) (bool, error) {
	return name == existingILM, nil
}

func (h *mockClientHandler) CreateILMPolicy(policy libilm.Policy) error {
	h.policies = append(h.policies, policy.Name)
	return nil
}
