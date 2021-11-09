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

	"github.com/pkg/errors"

	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"

	"github.com/elastic/apm-server/idxmgmt/common"
	"github.com/elastic/apm-server/idxmgmt/ilm"
)

const (
	// SetupDeprecatedWarning holds the warning message to display to users
	// when setting up index management and/or pipelines.
	SetupDeprecatedWarning = `WARNING: setting up Elasticsearch directly with apm-server is deprecated, and will be removed in 8.0.
New installations are encouraged to use the Elastic Agent integration. For more details, refer to
https://www.elastic.co/guide/en/apm/server/current/breaking-changes.html#_7_16`

	msgIlmDisabledES             = "Automatically disabled ILM as configured Elasticsearch not eligible for auto enabling."
	msgIlmDisabledCfg            = "Automatically disabled ILM as custom index settings configured."
	msgIdxCfgIgnored             = "Custom index configuration ignored when ILM is enabled."
	msgIlmSetupDisabled          = "Manage ILM setup is disabled."
	msgIlmSetupOverwriteDisabled = "Overwrite ILM setup is disabled."
	msgTemplateSetupDisabled     = "Template loading is disabled."
)

type manager struct {
	supporter     *supporter
	clientHandler libidxmgmt.ClientHandler
	assets        libidxmgmt.Asseter
}

// VerifySetup provides an opportunity to print a warning message to the console,
// for users who are running apm-server interactively.
func (m *manager) VerifySetup(loadTemplate, loadILM libidxmgmt.LoadMode) (bool, string) {
	warnings := []string{"\n" + SetupDeprecatedWarning + "\n"}
	templateFeature := m.templateFeature(loadTemplate)
	if _, _, ilmWarn, _, err := m.ilmFeature(loadILM); err != nil {
		warnings = append(warnings, err.Error())
	} else {
		if !templateFeature.load {
			warnings = append(warnings, msgTemplateSetupDisabled)
		}
		if ilmWarn != "" {
			warnings = append(warnings, ilmWarn)
		}
	}
	return false, strings.Join(warnings, " ")
}

// Setup is called for new Elasticsearch connections to ensure indices and templates are setup.
func (m *manager) Setup(loadTemplate, loadILM libidxmgmt.LoadMode) error {

	log := m.supporter.log

	//setup index management:
	//(0) preparation step
	//(1) load general apm template

	// if `apm-server.ilm.setup.managed=true`
	//(2) load policy per event type
	//(3) create template per event respecting lifecycle settings
	//(4) load write alias per event type AFTER the template has been created,
	//    as this step also automatically creates an index, it is important the matching templates are already there

	//(0) prepare template and ilm handlers, check if ILM is supported, fall back to ordinary index handling otherwise
	ilmFeature, ilmSupporters, warn, info, err := m.ilmFeature(loadILM)
	if err != nil {
		return err
	}
	if info != "" {
		log.Info(info)
	}
	if warn != "" {
		log.Warn(warn)
	}
	m.supporter.ilmEnabled.Store(ilmFeature.enabled)

	//(1) load general apm template
	templateFeature := m.templateFeature(loadTemplate)
	m.supporter.templateConfig.Enabled = templateFeature.enabled
	m.supporter.templateConfig.Overwrite = templateFeature.overwrite
	if err := m.loadTemplate(templateFeature, ilmFeature); err != nil {
		return err
	}

	if !ilmFeature.load {
		return nil
	}
	policiesLoaded := make(map[string]bool)
	for _, ilmSupporter := range ilmSupporters {
		//(2) load event type policies, respecting ILM settings
		if err := m.loadPolicy(ilmFeature, ilmSupporter, policiesLoaded); err != nil {
			return err
		}

		// (3) load event type specific template respecting index lifecycle information
		if err := m.loadEventTemplate(ilmFeature, ilmSupporter); err != nil {
			return err
		}

		//(4) load ilm write aliases
		//    ensure write aliases are created AFTER template creation
		if err := m.loadAlias(ilmFeature, ilmSupporter); err != nil {
			return err
		}
	}

	log.Info("Finished index management setup.")
	return nil
}

func (m *manager) templateFeature(loadMode libidxmgmt.LoadMode) feature {
	return newFeature(
		m.supporter.templateConfig.Enabled,
		m.supporter.templateConfig.Overwrite,
		m.supporter.templateConfig.Enabled,
		loadMode,
	)
}

func (m *manager) ilmFeature(loadMode libidxmgmt.LoadMode) (_ feature, _ []libilm.Supporter, warn, info string, _ error) {
	var ilmEnabled bool
	ilmSupporters := ilm.MakeDefaultSupporter(m.supporter.log, m.supporter.ilmConfig)
	if m.supporter.ilmConfig.Enabled {
		checkSupported := true
		if m.supporter.outputConfig.Name() != esKey {
			// Output is not Elasticsearch: ILM is disabled.
			warn += msgIlmDisabledES
			checkSupported = false
		} else if m.supporter.unmanagedIdxConfig.Customized() {
			if m.supporter.ilmConfig.Enabled {
				warn += msgIdxCfgIgnored
			} else {
				checkSupported = false
			}
		}
		if checkSupported && len(ilmSupporters) > 0 {
			// Check if ILM is supported by Elasticsearch.
			supporter := ilmSupporters[0]
			enabled, err := supporter.Manager(m.clientHandler).CheckEnabled()
			if err != nil {
				return feature{}, nil, "", "", err
			}
			ilmEnabled = enabled
			if !ilmEnabled {
				warn += msgIlmDisabledES
			}
		}
	}

	f := newFeature(
		ilmEnabled,
		m.supporter.ilmConfig.Setup.Overwrite,
		m.supporter.ilmConfig.Setup.Enabled,
		loadMode,
	)
	if !f.load {
		warn += msgIlmSetupDisabled
	}
	if !f.overwrite {
		info = msgIlmSetupOverwriteDisabled
	}
	return f, ilmSupporters, warn, info, nil
}

func (m *manager) loadTemplate(templateFeature, ilmFeature feature) error {
	if !templateFeature.load {
		return nil
	}
	// if not customized, set the APM template name and pattern to the
	// default index prefix for managed and unmanaged indices;
	// in case the index/rollover_alias names were customized
	if m.supporter.templateConfig.Name == "" && m.supporter.templateConfig.Pattern == "" {
		m.supporter.templateConfig.Name = common.APMPrefix
		m.supporter.log.Infof("Set setup.template.name to '%s'.", m.supporter.templateConfig.Name)
		m.supporter.templateConfig.Pattern = m.supporter.templateConfig.Name + "*"
		m.supporter.log.Infof("Set setup.template.pattern to '%s'.", m.supporter.templateConfig.Pattern)
	}
	if err := m.clientHandler.Load(
		m.supporter.templateConfig,
		m.supporter.info,
		m.assets.Fields(m.supporter.info.Beat),
		false, // migration
	); err != nil {
		return fmt.Errorf("error loading Elasticsearch template: %+v", err)
	}
	m.supporter.log.Infof("Finished loading index template.")
	return nil
}

func (m *manager) loadEventTemplate(feature feature, ilmSupporter libilm.Supporter) error {
	templateCfg := ilm.Template(feature.enabled, feature.overwrite,
		ilmSupporter.Alias().Name,
		ilmSupporter.Policy().Name)

	if err := m.clientHandler.Load(
		templateCfg,
		m.supporter.info,
		nil,   // fields
		false, // migration
	); err != nil {
		return errors.Wrapf(err, "error loading template %+v", templateCfg.Name)
	}
	m.supporter.log.Infof("Finished template setup for %s.", templateCfg.Name)
	return nil
}

func (m *manager) loadPolicy(ilmFeature feature, ilmSupporter libilm.Supporter, policiesLoaded map[string]bool) error {
	if !ilmFeature.enabled {
		return nil
	}
	policy := ilmSupporter.Policy().Name
	if policiesLoaded[policy] {
		return nil
	}
	if ilmSupporter.Policy().Body == nil {
		m.supporter.log.Infof("ILM policy %s not loaded.", policy)
		return nil
	}
	if _, err := ilmSupporter.Manager(m.clientHandler).EnsurePolicy(ilmFeature.overwrite); err != nil {
		return err
	}
	m.supporter.log.Infof("ILM policy %s successfully loaded.", policy)
	policiesLoaded[policy] = true
	return nil
}

func (m *manager) loadAlias(ilmFeature feature, ilmSupporter libilm.Supporter) error {
	if !ilmFeature.enabled {
		return nil
	}
	alias := ilmSupporter.Alias().Name
	if err := ilmSupporter.Manager(m.clientHandler).EnsureAlias(); err != nil {
		if libilm.ErrReason(err) != libilm.ErrAliasAlreadyExists {
			return err
		}
		m.supporter.log.Infof("Write alias %s exists already.", alias)
		return nil
	}
	m.supporter.log.Infof("Write alias %s successfully generated.", alias)
	return nil
}
