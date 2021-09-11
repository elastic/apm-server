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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/v7/libbeat/template"
)

func TestManager_VerifySetup(t *testing.T) {
	for name, tc := range map[string]struct {
		templateEnabled       bool
		ilmSetupEnabled       bool
		ilmSetupOverwrite     bool
		ilmEnabled            string
		loadTemplate, loadILM libidxmgmt.LoadMode
		version               string
		esCfg                 common.MapStr

		warn string
	}{
		"SetupTemplateDisabled": {
			loadTemplate: libidxmgmt.LoadModeEnabled,
			warn:         "Template loading is disabled",
		},
		"SetupILMDisabled": {
			loadILM:         libidxmgmt.LoadModeEnabled,
			ilmSetupEnabled: false,
			warn:            "Manage ILM setup is disabled.",
		},
		"LoadILMDisabled": {
			loadILM:           libidxmgmt.LoadModeDisabled,
			ilmSetupOverwrite: true,
			warn:              "Manage ILM setup is disabled.",
		},
		"LoadTemplateDisabled": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeDisabled,
			warn: "Template loading is disabled",
		},
		"ILMEnabledButUnsupported": {
			version:    "6.2.0",
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
		},
		"ILMAutoButUnsupported": {
			version: "6.2.0",
			loadILM: libidxmgmt.LoadModeEnabled,
			warn:    msgIlmDisabledES,
		},
		"ILMAutoCustomIndex": {
			loadILM: libidxmgmt.LoadModeEnabled,
			esCfg:   common.MapStr{"output.elasticsearch.index": "custom"},
			warn:    msgIlmDisabledCfg,
		},
		"ILMAutoCustomIndices": {
			loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.indices": []common.MapStr{{
				"index": "apm-custom-%{[observer.version]}-metric",
				"when": map[string]interface{}{
					"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			warn: msgIlmDisabledCfg,
		},
		"ILMTrueCustomIndex": {
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{"output.elasticsearch.index": "custom"},
			warn:  msgIdxCfgIgnored,
		},
		"LogstashOutput": {
			ilmEnabled: "true", loadILM: libidxmgmt.LoadModeEnabled,
			esCfg: common.MapStr{
				"output.elasticsearch.enabled": false,
				"output.logstash.enabled":      true},
			warn: "Automatically disabled ILM",
		},
		"EverythingEnabled": {
			templateEnabled: true, loadTemplate: libidxmgmt.LoadModeEnabled,
			ilmSetupEnabled: true, ilmSetupOverwrite: true, loadILM: libidxmgmt.LoadModeEnabled,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := common.MapStr{
				"setup.template.enabled":         tc.templateEnabled,
				"apm-server.ilm.setup.enabled":   tc.ilmSetupEnabled,
				"apm-server.ilm.setup.overwrite": tc.ilmSetupOverwrite,
			}
			if tc.ilmEnabled != "" {
				c["apm-server.ilm.enabled"] = tc.ilmEnabled
			}
			if tc.esCfg != nil {
				c.DeepUpdate(tc.esCfg)
			}
			support := defaultSupporter(t, c)
			version := tc.version
			if version == "" {
				version = "7.0.0"
			}
			manager := support.Manager(newMockClientHandler(version), nil)
			ok, warn := manager.VerifySetup(tc.loadTemplate, tc.loadILM)
			require.False(t, ok, warn)
			assert.Contains(t, warn, SetupDeprecatedWarning)
			assert.Contains(t, warn, tc.warn)
		})
	}
}

func TestManager_SetupTemplate(t *testing.T) {
	fields := []byte("apm-server fields")

	type testCase struct {
		cfg      common.MapStr
		loadMode libidxmgmt.LoadMode

		templates           int
		overwrittenTemplate bool
	}

	var testCasesEnabledTemplate = map[string]testCase{
		"Default": {
			loadMode:  libidxmgmt.LoadModeEnabled,
			templates: 1,
		},
		"OverwriteTemplate": {
			cfg:       common.MapStr{"setup.template.overwrite": true},
			loadMode:  libidxmgmt.LoadModeEnabled,
			templates: 1, overwrittenTemplate: true,
		},
		"LoadModeOverwrite": {
			loadMode:  libidxmgmt.LoadModeOverwrite,
			templates: 1, overwrittenTemplate: true,
		},
		"LoadModeForce": {
			loadMode:  libidxmgmt.LoadModeForce,
			templates: 1, overwrittenTemplate: true,
		},
		"LoadModeUnset": {
			templates: 0,
		},
	}
	var testCasesDisabledTemplate = map[string]testCase{
		"DisabledTemplate": {
			cfg:       common.MapStr{"setup.template.enabled": false},
			loadMode:  libidxmgmt.LoadModeEnabled,
			templates: 0,
		},
		"OverwriteTemplate": {
			cfg:       common.MapStr{"setup.template.enabled": false, "setup.template.overwrite": true},
			loadMode:  libidxmgmt.LoadModeEnabled,
			templates: 0,
		},
		"DisabledTemplate LoadModeOverwrite": {
			cfg:       common.MapStr{"setup.template.enabled": false},
			loadMode:  libidxmgmt.LoadModeOverwrite,
			templates: 0,
		},
		"DisabledTemplate LoadModeForce": {
			cfg:       common.MapStr{"setup.template.enabled": false},
			loadMode:  libidxmgmt.LoadModeForce,
			templates: 1, overwrittenTemplate: true,
		},
	}
	for _, test := range []map[string]testCase{testCasesEnabledTemplate, testCasesDisabledTemplate} {
		for name, tc := range test {
			t.Run(name, func(t *testing.T) {
				clientHandler := newMockClientHandler("8.0.0")
				m := defaultSupporter(t, tc.cfg).Manager(clientHandler, libidxmgmt.BeatsAssets(fields))
				indexManager := m.(*manager)
				require.NoError(t, indexManager.Setup(tc.loadMode, libidxmgmt.LoadModeDisabled))

				require.Equal(t, tc.templates, clientHandler.templates, "loaded template")
				assert.Equal(t, 0, clientHandler.templatesILMOrder, "order template")
				assert.Equal(t, tc.overwrittenTemplate, clientHandler.templateForceLoad, "overwritten template")
			})
		}
	}
}

func TestManager_SetupILM(t *testing.T) {
	fields := []byte("apm-server fields")

	type testCase struct {
		cfg      common.MapStr
		loadMode libidxmgmt.LoadMode

		err                                       string
		templatesILMEnabled, templatesILMDisabled int
		policiesLoaded, aliasesLoaded             int
		version                                   string
	}

	mappingRollover1Day := common.MapStr{"event_type": "error", "policy_name": "rollover-1-day"}
	policyRollover1Day := common.MapStr{
		"name": "rollover-1-day",
		"policy": common.MapStr{
			"phases": common.MapStr{
				"delete": common.MapStr{
					"actions": common.MapStr{
						"delete": common.MapStr{},
					},
				},
			},
		},
	}

	var testCasesSetupEnabled = map[string]testCase{
		"Default": {
			loadMode:            libidxmgmt.LoadModeEnabled,
			templatesILMEnabled: 4, policiesLoaded: 1, aliasesLoaded: 4,
		},
		"ILM disabled": {
			cfg:                  common.MapStr{"apm-server.ilm.enabled": false},
			loadMode:             libidxmgmt.LoadModeEnabled,
			templatesILMDisabled: 4,
		},
		"ILM setup enabled no overwrite": {
			cfg: common.MapStr{
				"apm-server.ilm.setup.enabled":   true,
				"apm-server.ilm.setup.overwrite": false,
				"apm-server.ilm.setup.mapping":   []common.MapStr{mappingRollover1Day},
				"apm-server.ilm.setup.policies":  []common.MapStr{policyRollover1Day},
			},
			loadMode:            libidxmgmt.LoadModeEnabled,
			templatesILMEnabled: 4, policiesLoaded: 1, aliasesLoaded: 4,
		},
		"ILM overwrite": {
			cfg: common.MapStr{
				"apm-server.ilm.setup.overwrite": true,
				"apm-server.ilm.setup.mapping":   []common.MapStr{mappingRollover1Day},
				"apm-server.ilm.setup.policies":  []common.MapStr{policyRollover1Day},
			},
			loadMode:            libidxmgmt.LoadModeEnabled,
			templatesILMEnabled: 5, policiesLoaded: 2, aliasesLoaded: 5,
		},
		"LoadModeOverwrite": {
			loadMode:            libidxmgmt.LoadModeOverwrite,
			templatesILMEnabled: 5, policiesLoaded: 1, aliasesLoaded: 4,
		},
		"LoadModeForce ILM enabled": {
			loadMode:            libidxmgmt.LoadModeForce,
			templatesILMEnabled: 5, policiesLoaded: 1, aliasesLoaded: 4,
		},
		"LoadModeForce ILM disabled": {
			cfg:                  common.MapStr{"apm-server.ilm.enabled": false},
			loadMode:             libidxmgmt.LoadModeForce,
			templatesILMDisabled: 5,
		},
		"ILM overwrite LoadModeDisabled": {
			cfg:                 common.MapStr{"apm-server.ilm.setup.overwrite": true},
			loadMode:            libidxmgmt.LoadModeDisabled,
			templatesILMEnabled: 0, templatesILMDisabled: 0,
		},
		"LoadModeUnset": {
			templatesILMEnabled: 0, templatesILMDisabled: 0,
		},
	}

	var testCasesSetupDisabled = map[string]testCase{
		"SetupDisabled": {
			cfg:      common.MapStr{"apm-server.ilm.setup.enabled": false, "apm-server.ilm.setup.overwrite": true},
			loadMode: libidxmgmt.LoadModeEnabled,
		},
		"SetupDisabled ILM disabled": {
			cfg:      common.MapStr{"apm-server.ilm.setup.enabled": false, "apm-server.ilm.setup.overwrite": true, "apm-server.ilm.enabled": false},
			loadMode: libidxmgmt.LoadModeEnabled,
		},
		"SetupDisabled LoadModeOverwrite": {
			cfg:      common.MapStr{"apm-server.ilm.setup.enabled": false, "apm-server.ilm.setup.overwrite": true},
			loadMode: libidxmgmt.LoadModeOverwrite,
		},
		"SetupDisabled LoadModeForce ILM enabled": {
			cfg:                 common.MapStr{"apm-server.ilm.setup.enabled": false},
			loadMode:            libidxmgmt.LoadModeForce,
			templatesILMEnabled: 5, policiesLoaded: 1, aliasesLoaded: 4,
		},
		"SetupDisabled LoadModeForce ILM disabled": {
			cfg:                  common.MapStr{"apm-server.ilm.setup.enabled": false, "apm-server.ilm.enabled": false},
			loadMode:             libidxmgmt.LoadModeForce,
			templatesILMDisabled: 5,
		},
		"LoadModeDisabled": {
			loadMode: libidxmgmt.LoadModeDisabled,
		},
	}

	var testCasesILMNotSupportedByES = map[string]testCase{
		"Default ES Unsupported ILM": {
			version:              "6.2.0",
			loadMode:             libidxmgmt.LoadModeEnabled,
			templatesILMDisabled: 4,
		},
		"SetupOverwrite Default ES Unsupported ILM": {
			cfg:                  common.MapStr{"apm-server.ilm.setup.overwrite": "true"},
			version:              "6.2.0",
			loadMode:             libidxmgmt.LoadModeEnabled,
			templatesILMDisabled: 5,
		},
		"ILM True ES Unsupported ILM": {
			cfg:                  common.MapStr{"apm-server.ilm.enabled": "true"},
			loadMode:             libidxmgmt.LoadModeEnabled,
			version:              "6.2.0",
			templatesILMDisabled: 4,
			err:                  "ILM not supported",
		},
		"Default ES Unsupported ILM setup disabled": {
			cfg:      common.MapStr{"apm-server.ilm.setup.enabled": false},
			loadMode: libidxmgmt.LoadModeEnabled,
			version:  "6.2.0",
		},
		"ILM True ES Unsupported ILM setup disabled": {
			cfg:      common.MapStr{"apm-server.ilm.setup.enabled": false, "apm-server.ilm.enabled": true},
			loadMode: libidxmgmt.LoadModeEnabled,
			version:  "6.2.0",
			err:      "ILM not supported",
		},
	}
	var testCasesILMNotSupportedByIndexSettings = map[string]testCase{
		"ESIndexConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":       "auto",
				"apm-server.ilm.setup.enabled": true,
				"setup.template.name":          "custom",
				"setup.template.pattern":       "custom",
				"output.elasticsearch.index":   "custom"},
			loadMode:             libidxmgmt.LoadModeEnabled,
			templatesILMDisabled: 4,
		},
		"ESIndicesConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":       "auto",
				"apm-server.ilm.setup.enabled": true,
				"setup.template.name":          "custom",
				"setup.template.pattern":       "custom",
				"output.elasticsearch.indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			loadMode:             libidxmgmt.LoadModeEnabled,
			templatesILMDisabled: 4,
		},
		"ESIndexConfigured setup disabled": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":       "auto",
				"apm-server.ilm.setup.enabled": false,
				"setup.template.name":          "custom",
				"setup.template.pattern":       "custom",
				"output.elasticsearch.index":   "custom"},
			loadMode: libidxmgmt.LoadModeEnabled,
		},
		"ESIndicesConfigured setup disabled": {
			cfg: common.MapStr{
				"apm-server.ilm.enabled":       "auto",
				"apm-server.ilm.setup.enabled": false,
				"setup.template.name":          "custom",
				"setup.template.pattern":       "custom",
				"output.elasticsearch.indices": []common.MapStr{{
					"index": "apm-custom-%{[observer.version]}-metric",
					"when": map[string]interface{}{
						"contains": map[string]interface{}{"processor.event": "metric"}}}}},
			loadMode: libidxmgmt.LoadModeEnabled,
		},
	}

	var testCasesPolicyNotConfigured = map[string]testCase{
		"policyNotConfigured": {
			cfg: common.MapStr{
				"apm-server.ilm.setup": map[string]interface{}{
					"require_policy": false,
					"mapping": []map[string]string{
						{"event_type": "error", "policy_name": "foo"},
						{"event_type": "transaction", "policy_name": "bar"}},
				}},
			loadMode: libidxmgmt.LoadModeEnabled,
			// templates for all event types are loaded
			// profile, span, and metrics share the same default policy, one policy is loaded
			// 1 alias already exists, 4 new ones are loaded
			templatesILMEnabled: 4, policiesLoaded: 1, aliasesLoaded: 4,
		},
	}

	for _, test := range []map[string]testCase{
		testCasesSetupEnabled,
		testCasesSetupDisabled,
		testCasesILMNotSupportedByES,
		testCasesILMNotSupportedByIndexSettings,
		testCasesPolicyNotConfigured,
	} {
		for name, tc := range test {
			t.Run(name, func(t *testing.T) {
				version := tc.version
				if version == "" {
					version = "8.0.0"
				}
				clientHandler := newMockClientHandler(version)
				m := defaultSupporter(t, tc.cfg).Manager(clientHandler, libidxmgmt.BeatsAssets(fields))
				indexManager := m.(*manager)
				err := indexManager.Setup(libidxmgmt.LoadModeDisabled, tc.loadMode)
				if tc.err != "" {
					require.EqualError(t, err, tc.err)
				} else {
					require.NoError(t, err)
					assert.Len(t, clientHandler.policies, tc.policiesLoaded)
					assert.Len(t, clientHandler.aliases, tc.aliasesLoaded)
					require.Equal(t, tc.templatesILMEnabled, clientHandler.templatesILMEnabled, "ILM enabled templates")
					require.Equal(t, tc.templatesILMDisabled, clientHandler.templates, "ILM disabled templates")
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

	templates, templatesILMEnabled int
	templatesILMOrder              int
	templateForceLoad              bool

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
	if config.Settings.Index != nil && config.Settings.Index["lifecycle.name"] != nil {
		h.templatesILMEnabled++
		if len(fields) > 0 {
			return errors.New("fields should be empty")
		}
	} else {
		h.templates++
	}
	if config.Order == 2 {
		h.templatesILMOrder++
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
	if alias.Name == existingILMAlias {
		return reasonedError{
			error:  errors.New("CreateAlias failed"),
			reason: libilm.ErrAliasAlreadyExists,
		}
	}
	return nil
}

func (h *mockClientHandler) HasILMPolicy(name string) (bool, error) {
	return name == existingILMPolicy, nil
}

func (h *mockClientHandler) CreateILMPolicy(policy libilm.Policy) error {
	h.policies = append(h.policies, policy.Name)
	return nil
}

type reasonedError struct {
	error
	reason error
}

func (e reasonedError) Reason() error {
	return e.reason
}
