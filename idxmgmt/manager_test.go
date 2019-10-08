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
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/template"
)

func TestManager_VerifySetup(t *testing.T) {
	for name, setup := range map[string]struct {
		tmpl              bool
		ilm               string
		loadTmpl, loadILM libidxmgmt.LoadMode
		version           string
		esCfg             common.MapStr

		ok   bool
		warn string
	}{
		"LoadTemplateWithoutILM": {
			ilm: "true", loadILM: libidxmgmt.LoadModeDisabled,
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			warn: "whithout loading ILM policy and alias",
		},
		"LoadILMWithoutTemplate": {
			ilm: "true", loadILM: libidxmgmt.LoadModeEnabled,
			loadTmpl: libidxmgmt.LoadModeEnabled,
			warn:     "without loading template is not recommended",
		},
		"SetupTemplateDisabled": {
			ilm: "false", loadILM: libidxmgmt.LoadModeEnabled,
			loadTmpl: libidxmgmt.LoadModeEnabled,
			warn:     "loading not enabled",
		},
		"SetupILMDisabled": {
			ilm: "false", loadILM: libidxmgmt.LoadModeEnabled,
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			warn: "loading not enabled",
		},
		"LoadILMDisabled": {
			ilm: "true", loadILM: libidxmgmt.LoadModeDisabled,
			loadTmpl: libidxmgmt.LoadModeEnabled,
			warn:     "loading not enabled",
		},
		"LoadTemplateDisabled": {
			ilm: "false", loadILM: libidxmgmt.LoadModeEnabled,
			tmpl: true, loadTmpl: libidxmgmt.LoadModeDisabled,
			warn: "loading not enabled",
		},
		"ILMEnabledButUnsupported": {
			version: "6.2.0",
			ilm:     "true", loadILM: libidxmgmt.LoadModeEnabled,
			loadTmpl: libidxmgmt.LoadModeEnabled,
			warn:     msgErrIlmDisabledES,
		},
		"ILMAutoButUnsupported": {
			loadTmpl: libidxmgmt.LoadModeEnabled,
			version:  "6.2.0",
			tmpl:     true,
			ilm:      "auto", loadILM: libidxmgmt.LoadModeEnabled,
			warn: msgIlmDisabledES,
		},
		"ILMAutoCustomIndex": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			ilm: "auto", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.index": "custom"},
			warn:  msgIlmDisabledCfg,
		},
		"ILMAutoCustomIndices": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			ilm: "auto", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.indices": []common.MapStr{{
				"index": "apm-custom-%{[observer.version]}-metric",
				"when": map[string]interface{}{
					"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			warn: msgIlmDisabledCfg,
		},
		"ILMTrueCustomIndex": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			ilm: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.index": "custom"},
			warn:  msgIdxCfgIgnored,
		},
		"LogstashOutput": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			ilm: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{
				"output.elasticsearch.enabled": false,
				"output.logstash.enabled":      true},
			warn: "automatically disabled ILM",
		},
		"EverythingEnabled": {
			tmpl: true, loadTmpl: libidxmgmt.LoadModeEnabled,
			ilm: "true", loadILM: libidxmgmt.LoadModeEnabled,
			ok: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := common.MapStr{
				"apm-server.ilm.enabled": setup.ilm,
				"setup.template.enabled": setup.tmpl,
			}
			if setup.esCfg != nil {
				c.DeepUpdate(setup.esCfg)
			}
			support := defaultSupporter(t, c)
			version := setup.version
			if version == "" {
				version = "7.0.0"
			}
			manager := support.Manager(newMockClientHandler(version), nil)
			ok, warn := manager.VerifySetup(setup.loadTmpl, setup.loadILM)
			require.Equal(t, setup.ok, ok, warn)
			assert.Contains(t, warn, setup.warn)
		})
	}
}

func TestManager_Setup(t *testing.T) {
	fields := []byte("apm-server fields")

	type testCase struct {
		cfg            common.MapStr
		tLoad, ilmLoad libidxmgmt.LoadMode

		tCt, tWithILMCt, pCt, aCt int
		version                   string
		err                       error
	}

	var testCasesEnabled = map[string]testCase{
		"Default": {
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 2, aCt: 3,
		},
		"LoadTemplateAndILM": {
			cfg:   common.MapStr{"apm-server.ilm.enabled": true},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 2, aCt: 3,
		},
		"DisabledILM": {
			cfg:   common.MapStr{"apm-server.ilm.enabled": false},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 4,
		},
		"DisabledTemplate": {
			cfg:   common.MapStr{"setup.template.enabled": false},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			pCt: 2, aCt: 3,
		},
		"DisabledTemplateAndILM": {
			cfg:   common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
		},
		"LoadModeDisabledILM": {
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeDisabled,
			tCt: 4, tWithILMCt: 3,
		},
		"LoadModeDisabledTemplate": {
			tLoad: libidxmgmt.LoadModeDisabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			pCt: 2, aCt: 3,
		},
		"LoadModeDisabledTemplateAndILM": {
			tLoad:   libidxmgmt.LoadModeDisabled,
			ilmLoad: libidxmgmt.LoadModeDisabled,
		},
	}

	var testCasesOverwrite = map[string]testCase{
		"OverwriteILM": {
			cfg:   common.MapStr{"apm-server.ilm.overwrite": true},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 4, aCt: 3,
		},
		"OverwriteTemplate": {
			cfg:   common.MapStr{"setup.template.overwrite": true},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 2, aCt: 3,
		},
		"OverwriteTemplateAndILM": {
			cfg:   common.MapStr{"setup.template.overwrite": true, "apm-server.ilm.overwrite": true},
			tLoad: libidxmgmt.LoadModeEnabled, ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt: 5, tWithILMCt: 4, pCt: 4, aCt: 3,
		},
	}

	var testCasesLoadModeUnset = map[string]testCase{
		"DefaultLoadModeUnset": {
			tCt: 0, tWithILMCt: 0, pCt: 0, aCt: 0,
		},
	}

	var testCasesLoadModeOverwrite = map[string]testCase{
		// used for `./apm-server setup template`, `apm-server setup index-managemet`

		"DisabledILMAndTemplateLoadModeOverwrite": {
			cfg:     common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			tLoad:   libidxmgmt.LoadModeOverwrite,
			ilmLoad: libidxmgmt.LoadModeOverwrite,
		},
		"LoadModeOverwriteTemplateLoadModeDisabledILM": {
			// used for `./apm-server setup template`
			tLoad:   libidxmgmt.LoadModeOverwrite,
			ilmLoad: libidxmgmt.LoadModeDisabled,
			tCt:     5, tWithILMCt: 4,
		},
		"LoadModeOverwriteILMLoadModeDisabledTemplate": {
			tLoad:   libidxmgmt.LoadModeDisabled,
			ilmLoad: libidxmgmt.LoadModeOverwrite,
			pCt:     4, aCt: 3,
		},
		"LoadModeOverwrite": {
			// used for `./apm-server setup template`, `apm-server setup index-managemet`
			tLoad:   libidxmgmt.LoadModeOverwrite,
			ilmLoad: libidxmgmt.LoadModeOverwrite,
			tCt:     5, tWithILMCt: 4, pCt: 4, aCt: 3,
		},
	}

	var testCasesLoadModeForce = map[string]testCase{
		// used for `./apm-server export template`

		"LoadModeForceTemplateLoadModeDisabledILM": {
			cfg:     common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			tLoad:   libidxmgmt.LoadModeForce,
			ilmLoad: libidxmgmt.LoadModeDisabled,
			tCt:     5,
		},
		"LoadModeForceILMLoadModeDisabledTemplate": {
			// used for `./apm-server export ilm-policy`
			cfg:     common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			tLoad:   libidxmgmt.LoadModeDisabled,
			ilmLoad: libidxmgmt.LoadModeForce,
			pCt:     4, aCt: 3,
		},
		"LoadModeForceWhenDisabled": {
			cfg:     common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			tLoad:   libidxmgmt.LoadModeForce,
			ilmLoad: libidxmgmt.LoadModeForce,
			tCt:     5, tWithILMCt: 4, pCt: 4, aCt: 3,
		},
	}

	var testCasesILMNotSupportedByES = map[string]testCase{
		"DisabledAndUnsupportedILM": {
			version: "6.2.0",
			cfg:     common.MapStr{"apm-server.ilm.enabled": false},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4,
		},
		"AutoAndUnsupportedILM": {
			version: "6.2.0",
			cfg:     common.MapStr{"apm-server.ilm.enabled": "auto"},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4,
		},
		"EnabledAndUnsupportedILM": {
			version: "6.2.0",
			cfg:     common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4,
		},
		"ForceModeWhenUnsupportedILM": {
			version: "6.2.0",
			cfg:     common.MapStr{"apm-server.ilm.enabled": true},
			tLoad:   libidxmgmt.LoadModeForce,
			ilmLoad: libidxmgmt.LoadModeForce,
			tCt:     5,
		},
	}
	var testCasesILMNotSupportedByIndexSettings = map[string]testCase{
		"ESIndexConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":     "auto",
				"setup.template.name":        "custom",
				"setup.template.pattern":     "custom",
				"output.elasticsearch.index": "custom"},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4,
		},
		"ESIndicesConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled": "auto",
				"setup.template.name":    "custom",
				"setup.template.pattern": "custom",
				"output.elasticsearch.indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4,
		},
	}

	var testCasesPolicyNotConfigured = map[string]testCase{
		"policyNotConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm": map[string]interface{}{
					"require_policy": false,
					"mapping": []map[string]string{
						{"event_type": "error", "policy": "foo"},
						{"event_type": "transaction", "policy": "bar"}},
				}},
			tLoad:   libidxmgmt.LoadModeEnabled,
			ilmLoad: libidxmgmt.LoadModeEnabled,
			tCt:     4, tWithILMCt: 3, pCt: 1, aCt: 3,
		},
	}

	for _, test := range []map[string]testCase{
		testCasesEnabled,
		testCasesOverwrite,
		testCasesLoadModeUnset,
		testCasesLoadModeOverwrite,
		testCasesLoadModeForce,
		testCasesILMNotSupportedByES,
		testCasesILMNotSupportedByIndexSettings,
		testCasesPolicyNotConfigured,
	} {
		for name, tc := range test {
			t.Run(name, func(t *testing.T) {
				version := tc.version
				if version == "" {
					version = "7.2.0"
				}

				ch := newMockClientHandler(version)
				m := defaultSupporter(t, tc.cfg).Manager(ch, libidxmgmt.BeatsAssets(fields))
				idxManager := m.(*manager)
				err := idxManager.Setup(tc.tLoad, tc.ilmLoad)
				if tc.err == nil {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Equal(t, ErrILMNotSupported, err)
				}

				assert.Equal(t, tc.pCt, len(ch.policies), "policies")
				assert.Equal(t, tc.aCt, len(ch.aliases), "aliases")
				require.Equal(t, tc.tCt, ch.tmplLoadCt, "templates")
				assert.Equal(t, tc.tWithILMCt, ch.tmplLoadWithILMCt, "ILM templates")
				if tc.tCt > 0 {
					assert.Equal(t, 1, ch.tmplLoadCt-ch.tmplOrderILMCt, "order template")
					assert.Equal(t, tc.tCt-1, ch.tmplOrderILMCt, "order ILM template")
				}
			})
		}
	}
}

type mockClientHandler struct {
	// mockClientHandler loads templates, ilm templates, policies and aliases
	// The handler generally treats them as non-existing in Elasticsearch.
	// There are some exceptions to this rule to simulate the behavior related to the `overwrite` flag
	// Existing instances are:
	// * transaction event type specific template
	// * transaction ilm alias
	// * rollover-1-day policy

	aliases, policies []string

	tmplLoadCt, tmplLoadWithILMCt int
	tmplOrderILMCt                int
	tmplForce                     bool

	esVersion *common.Version
}

var existingILMAlias = fmt.Sprintf("apm-%s-transaction", info.Version)
var existingILMPolicy = "rollover-1-day"
var ErrILMNotSupported = errors.New("ILM not supported")
var esMinILMVersion = common.MustNewVersion("6.6.0")

func newMockClientHandler(esVersion string) *mockClientHandler {
	return &mockClientHandler{esVersion: common.MustNewVersion(esVersion)}
}

func (h *mockClientHandler) Load(config template.TemplateConfig, _ beat.Info, fields []byte, migration bool) error {
	if strings.Contains(config.Name, "transaction") && !config.Overwrite {
		return nil
	}
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

func (h *mockClientHandler) CheckILMEnabled(mode libilm.Mode) (bool, error) {
	if mode == libilm.ModeDisabled {
		return false, nil
	}
	avail := !h.esVersion.LessThan(esMinILMVersion)
	if avail {
		return true, nil
	}

	if mode == libilm.ModeAuto {
		return false, nil
	}
	return false, ErrILMNotSupported
}

func (h *mockClientHandler) HasAlias(name string) (bool, error) {
	return name == existingILMAlias, nil
}

func (h *mockClientHandler) CreateAlias(alias libilm.Alias) error {
	h.aliases = append(h.aliases, alias.Name)
	return nil
}

func (h *mockClientHandler) HasILMPolicy(name string) (bool, error) {
	fmt.Println(name)
	return name == existingILMPolicy, nil
}

func (h *mockClientHandler) CreateILMPolicy(policy libilm.Policy) error {
	h.policies = append(h.policies, policy.Name)
	return nil
}
