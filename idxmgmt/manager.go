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

	"github.com/pkg/errors"

	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"

	"github.com/elastic/apm-server/idxmgmt/common"
	"github.com/elastic/apm-server/idxmgmt/ilm"
)

const (
	msgErrIlmDisabledES          = "automatically disabled ILM as not supported by configured Elasticsearch"
	msgIlmDisabledES             = "Automatically disabled ILM as configured Elasticsearch not eligible for auto enabling."
	msgIlmDisabledCfg            = "Automatically disabled ILM as custom index settings configured."
	msgIdxCfgIgnored             = "Custom index configuration ignored when ILM is enabled."
	msgIlmSetupDisabled          = "Manage ILM setup is disabled. "
	msgIlmSetupOverwriteDisabled = "Overwrite ILM setup is disabled. "
	msgTemplateSetupDisabled     = "Template loading is disabled. "
)

type manager struct {
	supporter     *supporter
	clientHandler libidxmgmt.ClientHandler
	assets        libidxmgmt.Asseter
}

func (m *manager) VerifySetup(loadTemplate, loadILM libidxmgmt.LoadMode) (bool, string) {
	templateFeature := m.templateFeature(loadTemplate)
	ilmFeature := m.ilmFeature(loadILM)

	if err := ilmFeature.error(); err != nil {
		return false, err.Error()
	}

	var warn string
	if !templateFeature.load {
		warn += msgTemplateSetupDisabled
	}
	if ilmWarn := ilmFeature.warning(); ilmWarn != "" {
		warn += ilmWarn
	}
	return warn == "", warn
}

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

	ilmFeature := m.ilmFeature(loadILM)
	if info := ilmFeature.information(); info != "" {
		log.Info(info)
	}
	if warn := ilmFeature.warning(); warn != "" {
		log.Warn(warn)
	}
	if err := ilmFeature.error(); err != nil {
		log.Error(err)
	}

	templateFeature := m.templateFeature(loadTemplate)
	m.supporter.templateConfig.Enabled = templateFeature.enabled
	m.supporter.templateConfig.Overwrite = templateFeature.overwrite

	//(1) load general apm template
	if err := m.loadTemplate(templateFeature, ilmFeature); err != nil {
		return err
	}

	if !ilmFeature.load {
		return nil
	}

	policiesLoaded := make(map[string]bool)
	var err error
	for _, ilmSupporter := range m.supporter.ilmSupporters {
		//(2) load event type policies, respecting ILM settings
		if err := m.loadPolicy(ilmFeature, ilmSupporter, policiesLoaded); err != nil {
			return err
		}

		// (3) load event type specific template respecting index lifecycle information
		if err = m.loadEventTemplate(ilmFeature, ilmSupporter); err != nil {
			return err
		}

		//(4) load ilm write aliases
		//    ensure write aliases are created AFTER template creation
		if err = m.loadAlias(ilmFeature, ilmSupporter); err != nil {
			return err
		}
	}

	log.Info("Finished index management setup.")
	return nil
}

func (m *manager) templateFeature(loadMode libidxmgmt.LoadMode) feature {
	return newFeature(m.supporter.templateConfig.Enabled, m.supporter.templateConfig.Overwrite,
		m.supporter.templateConfig.Enabled, true, loadMode)
}

func (m *manager) ilmFeature(loadMode libidxmgmt.LoadMode) feature {
	// Do not use configured `m.supporter.ilmConfig.Mode` to check if ilm is enabled.
	// The configuration might be set to `true` or `auto` but preconditions are not met,
	// e.g. ilm support by Elasticsearch
	// In these cases the supporter holds an internal state `m.supporter.st.ilmEnabled` that is set to false.
	// The originally configured value is preserved allowing to collect warnings and errors to be
	// returned to the user.

	warning := func(f feature) string {
		if !f.load {
			return msgIlmSetupDisabled
		}
		return ""
	}
	information := func(f feature) string {
		if !f.overwrite {
			return msgIlmSetupOverwriteDisabled
		}
		return ""
	}
	// m.supporter.st.ilmEnabled.Load() only returns true for cases where
	// ilm mode is configured `auto` or `true` and preconditions to enable ilm are true
	if enabled := m.supporter.st.ilmEnabled.Load(); enabled {
		f := newFeature(enabled, m.supporter.ilmConfig.Setup.Overwrite,
			m.supporter.ilmConfig.Setup.Enabled, true, loadMode)
		f.warn = warning(f)
		if m.supporter.unmanagedIdxConfig.Customized() {
			f.warn += msgIdxCfgIgnored
		}
		f.info = information(f)
		return f
	}

	var (
		err       error
		supported = true
	)
	// collect warnings when ilm is configured `auto` but it cannot be enabled
	// collect error when ilm is configured `true` but it cannot be enabled as preconditions are not met
	var warn string
	if m.supporter.ilmConfig.Mode == libilm.ModeAuto {
		if m.supporter.unmanagedIdxConfig.Customized() {
			warn = msgIlmDisabledCfg
		} else {
			warn = msgIlmDisabledES
			supported = false
		}
	} else if m.supporter.ilmConfig.Mode == libilm.ModeEnabled {
		err = errors.New(msgErrIlmDisabledES)
		supported = false
	}
	f := newFeature(false, m.supporter.ilmConfig.Setup.Overwrite, m.supporter.ilmConfig.Setup.Enabled, supported, loadMode)
	f.warn = warning(f)
	f.warn += warn
	f.info = information(f)
	f.err = err
	return f
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
	if err := m.clientHandler.Load(m.supporter.templateConfig, m.supporter.info,
		m.assets.Fields(m.supporter.info.Beat), m.supporter.migration); err != nil {
		return fmt.Errorf("error loading Elasticsearch template: %+v", err)
	}
	m.supporter.log.Infof("Finished loading index template.")
	return nil
}

func (m *manager) loadEventTemplate(feature feature, ilmSupporter libilm.Supporter) error {
	templateCfg := ilm.Template(feature.enabled, feature.overwrite,
		ilmSupporter.Alias().Name,
		ilmSupporter.Policy().Name)

	if err := m.clientHandler.Load(templateCfg, m.supporter.info, nil, m.supporter.migration); err != nil {
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
