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
		templateEnabled       bool
		ilmEnabled            string
		loadTemplate, loadILM libidxmgmt.LoadMode
		version               string
		esCfg                 common.MapStr

		ok   bool
		warn string
	}{
		"LoadTemplateWithoutILM": {
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeDisabled,
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			warn: "whithout loading ILM policy and alias",
		},
		"LoadILMWithoutTemplate": {
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			loadTemplate: libidxmgmt.LoadModeEnabled,
			warn:         "without loading template is not recommended",
		},
		"SetupTemplateDisabled": {
			ilmEnabled: "false", loadILM: libidxmgmt.LoadModeEnabled,
			loadTemplate: libidxmgmt.LoadModeEnabled,
			warn:         "loading not enabled",
		},
		"SetupILMDisabled": {
			ilmEnabled: "false", loadILM: libidxmgmt.LoadModeEnabled,
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			warn: "loading not enabled",
		},
		"LoadILMDisabled": {
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeDisabled,
			loadTemplate: libidxmgmt.LoadModeEnabled,
			warn:         "loading not enabled",
		},
		"LoadTemplateDisabled": {
			ilmEnabled: "false", loadILM: libidxmgmt.LoadModeEnabled,
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeDisabled,
			warn: "loading not enabled",
		},
		"ILMEnabledButUnsupported": {
			version:    "6.2.0",
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			loadTemplate: libidxmgmt.LoadModeEnabled,
			warn:         msgErrIlmDisabledES,
		},
		"ILMAutoButUnsupported": {
			loadTemplate:    libidxmgmt.LoadModeEnabled,
			version:         "6.2.0",
			templateEnabled: true,
			ilmEnabled:      "auto", loadILM: libidxmgmt.LoadModeEnabled,
			warn: msgIlmDisabledES,
		},
		"ILMAutoCustomIndex": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmEnabled: "auto", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.index": "custom"},
			warn:  msgIlmDisabledCfg,
		},
		"ILMAutoCustomIndices": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmEnabled: "auto", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.indices": []common.MapStr{{
				"index": "apm-custom-%{[observer.version]}-metric",
				"when": map[string]interface{}{
					"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			warn: msgIlmDisabledCfg,
		},
		"ILMTrueCustomIndex": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.index": "custom"},
			warn:  msgIdxCfgIgnored,
		},
		"LogstashOutput": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{
				"output.elasticsearch.enabled": false,
				"output.logstash.enabled":      true},
			warn: "automatically disabled ILM",
		},
		"EverythingEnabled": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			ok: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := common.MapStr{
				"apm-server.ilm.enabled": setup.ilmEnabled,
				"setup.template.enabled": setup.templateEnabled,
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
			ok, warn := manager.VerifySetup(setup.loadTemplate, setup.loadILM)
			require.Equal(t, setup.ok, ok, warn)
			assert.Contains(t, warn, setup.warn)
		})
	}
}

func TestManager_Setup(t *testing.T) {
	fields := []byte("apm-server fields")

	type testCase struct {
		cfg                   common.MapStr
		loadTemplate, loadILM libidxmgmt.LoadMode

		templateLoadedCt, templateWithILMLoadedCt int
		policyLoadedCt, aliasLoadedCt             int
		version                                   string
		err                                       error
	}

	var testCasesEnabled = map[string]testCase{
		"Default": {
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 2, aliasLoadedCt: 3,
		},
		"LoadTemplateAndILM": {
			cfg:          common.MapStr{"apm-server.ilm.enabled": true},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 2, aliasLoadedCt: 3,
		},
		"DisabledILM": {
			cfg:          common.MapStr{"apm-server.ilm.enabled": false},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
		},
		"DisabledTemplate": {
			cfg:          common.MapStr{"setup.template.enabled": false},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			policyLoadedCt: 2, aliasLoadedCt: 3,
		},
		"DisabledTemplateAndILM": {
			cfg:          common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
		},
		"LoadModeDisabledILM": {
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeDisabled,
			templateLoadedCt: 4, templateWithILMLoadedCt: 3,
		},
		"LoadModeDisabledTemplate": {
			loadTemplate: libidxmgmt.LoadModeDisabled, loadILM: libidxmgmt.LoadModeEnabled,
			policyLoadedCt: 2, aliasLoadedCt: 3,
		},
		"LoadModeDisabledTemplateAndILM": {
			loadTemplate: libidxmgmt.LoadModeDisabled,
			loadILM:      libidxmgmt.LoadModeDisabled,
		},
	}

	var testCasesOverwrite = map[string]testCase{
		"OverwriteILM": {
			cfg:          common.MapStr{"apm-server.ilm.overwrite": true},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 4, aliasLoadedCt: 3,
		},
		"OverwriteTemplate": {
			cfg:          common.MapStr{"setup.template.overwrite": true},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 2, aliasLoadedCt: 3,
		},
		"OverwriteTemplateAndILM": {
			cfg:          common.MapStr{"setup.template.overwrite": true, "apm-server.ilm.overwrite": true},
			loadTemplate: libidxmgmt.LoadModeEnabled, loadILM: libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 4, aliasLoadedCt: 3,
		},
	}

	var testCasesLoadModeUnset = map[string]testCase{
		"DefaultLoadModeUnset": {
			templateLoadedCt: 0, templateWithILMLoadedCt: 0, policyLoadedCt: 0, aliasLoadedCt: 0,
		},
	}

	var testCasesLoadModeOverwrite = map[string]testCase{
		// used for `./apm-server setup template`, `apm-server setup index-managemet`

		"DisabledILMAndTemplateLoadModeOverwrite": {
			cfg:          common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			loadTemplate: libidxmgmt.LoadModeOverwrite,
			loadILM:      libidxmgmt.LoadModeOverwrite,
		},
		"LoadModeOverwriteTemplateLoadModeDisabledILM": {
			// used for `./apm-server setup template`
			loadTemplate:     libidxmgmt.LoadModeOverwrite,
			loadILM:          libidxmgmt.LoadModeDisabled,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4,
		},
		"LoadModeOverwriteILMLoadModeDisabledTemplate": {
			loadTemplate:   libidxmgmt.LoadModeDisabled,
			loadILM:        libidxmgmt.LoadModeOverwrite,
			policyLoadedCt: 4, aliasLoadedCt: 3,
		},
		"LoadModeOverwrite": {
			// used for `./apm-server setup template`, `apm-server setup index-managemet`
			loadTemplate:     libidxmgmt.LoadModeOverwrite,
			loadILM:          libidxmgmt.LoadModeOverwrite,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 4, aliasLoadedCt: 3,
		},
	}

	var testCasesLoadModeForce = map[string]testCase{
		// used for `./apm-server export template`

		"LoadModeForceTemplateLoadModeDisabledILM": {
			cfg:              common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			loadTemplate:     libidxmgmt.LoadModeForce,
			loadILM:          libidxmgmt.LoadModeDisabled,
			templateLoadedCt: 5,
		},
		"LoadModeForceILMLoadModeDisabledTemplate": {
			// used for `./apm-server export ilm-policy`
			cfg:            common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			loadTemplate:   libidxmgmt.LoadModeDisabled,
			loadILM:        libidxmgmt.LoadModeForce,
			policyLoadedCt: 4, aliasLoadedCt: 3,
		},
		"LoadModeForceWhenDisabled": {
			cfg:              common.MapStr{"setup.template.enabled": false, "apm-server.ilm.enabled": false},
			loadTemplate:     libidxmgmt.LoadModeForce,
			loadILM:          libidxmgmt.LoadModeForce,
			templateLoadedCt: 5, templateWithILMLoadedCt: 4, policyLoadedCt: 4, aliasLoadedCt: 3,
		},
	}

	var testCasesILMNotSupportedByES = map[string]testCase{
		"DisabledAndUnsupportedILM": {
			version:          "6.2.0",
			cfg:              common.MapStr{"apm-server.ilm.enabled": false},
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
		},
		"AutoAndUnsupportedILM": {
			version:          "6.2.0",
			cfg:              common.MapStr{"apm-server.ilm.enabled": "auto"},
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
		},
		"EnabledAndUnsupportedILM": {
			version:          "6.2.0",
			cfg:              common.MapStr{"apm-server.ilm.enabled": true},
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
		},
		"ForceModeWhenUnsupportedILM": {
			version:          "6.2.0",
			cfg:              common.MapStr{"apm-server.ilm.enabled": true},
			loadTemplate:     libidxmgmt.LoadModeForce,
			loadILM:          libidxmgmt.LoadModeForce,
			templateLoadedCt: 5,
		},
	}
	var testCasesILMNotSupportedByIndexSettings = map[string]testCase{
		"ESIndexConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":     "auto",
				"setup.template.name":        "custom",
				"setup.template.pattern":     "custom",
				"output.elasticsearch.index": "custom"},
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
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
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4,
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
			loadTemplate:     libidxmgmt.LoadModeEnabled,
			loadILM:          libidxmgmt.LoadModeEnabled,
			templateLoadedCt: 4, templateWithILMLoadedCt: 3, policyLoadedCt: 1, aliasLoadedCt: 3,
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

				clientHandler := newMockClientHandler(version)
				m := defaultSupporter(t, tc.cfg).Manager(clientHandler, libidxmgmt.BeatsAssets(fields))
				indexManager := m.(*manager)
				err := indexManager.Setup(tc.loadTemplate, tc.loadILM)
				if tc.err == nil {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Equal(t, ErrILMNotSupported, err)
				}

				assert.Equal(t, tc.policyLoadedCt, len(clientHandler.policies), "policies")
				assert.Equal(t, tc.aliasLoadedCt, len(clientHandler.aliases), "aliases")
				require.Equal(t, tc.templateLoadedCt, clientHandler.templateLoadedCt, "templates")
				assert.Equal(t, tc.templateWithILMLoadedCt, clientHandler.templateWithILMLoadedCt, "ILM templates")
				if tc.templateLoadedCt > 0 {
					assert.Equal(t, 1, clientHandler.templateLoadedCt-clientHandler.templateILMOrderCt, "order template")
					assert.Equal(t, tc.templateLoadedCt-1, clientHandler.templateILMOrderCt, "order ILM template")
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

	templateLoadedCt, templateWithILMLoadedCt int
	templateILMOrderCt                        int
	templateForceLoad                         bool

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
	h.templateLoadedCt++
	if config.Settings.Index != nil && config.Settings.Index["lifecycle.name"] != nil {
		h.templateWithILMLoadedCt++
		if len(fields) > 0 {
			return errors.New("fields should be empty")
		}
	}
	if config.Order == 2 {
		h.templateILMOrderCt++
	}
	if config.Order != 1 && config.Order != 2 {
		return errors.New("unexpected template order")
	}
	h.templateForceLoad = config.Overwrite
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
	return name == existingILMPolicy, nil
}

func (h *mockClientHandler) CreateILMPolicy(policy libilm.Policy) error {
	h.policies = append(h.policies, policy.Name)
	return nil
}
