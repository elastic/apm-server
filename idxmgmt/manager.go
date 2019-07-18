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

	"github.com/elastic/apm-server/idxmgmt/ilm"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

const (
	msgErrIlmDisabledES = "automatically disabled ILM as not supported by configured Elasticsearch"
	msgIlmDisabledES    = "Automatically disabled ILM as configured Elasticsearch not eligible for auto enabling."
	msgIlmDisabledCfg   = "Automatically disabled ILM as custom index settings configured."
	msgIdxCfgIgnored    = "Custom index configuration ignored when ILM is enabled."
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

	if ilmFeature.load && !templateFeature.load {
		return false, "Loading ILM policy and write alias without loading template " +
			"is not recommended. Check your configuration."
	}
	if templateFeature.load && !ilmFeature.load && ilmFeature.enabled {
		return false, "Loading template with ILM settings whithout loading ILM policy and alias can lead " +
			"to issues and is not recommended. Check your configuration"
	}

	var warn string

	if !ilmFeature.load && ilmFeature.supported {
		warn = "ILM policy and write alias loading not enabled. "
	}
	if !templateFeature.load {
		warn += "Template loading not enabled. "
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
	//(2) load policy per event type
	//(3) create template per event respecting lifecycle settings
	//(4) load write alias per event type AFTER the template has been created,
	//    as this step also automatically creates an index, it is important the matching templates are already there

	//(0) prepare template and ilm handlers, check if ILM is supported, fall back to ordinary index handling otherwise

	ilmFeature := m.ilmFeature(loadILM)
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
	//only set to user configured name and pattern if ilm is disabled
	//default template name and pattern, must be the same whether or not ilm is enabled or not,
	//allowing former templates to be overwritten
	if err := m.loadTemplate(templateFeature, ilmFeature); err != nil {
		return err
	}

	for _, ilmSupporter := range m.supporter.ilmSupporters {
		//(2) load event type policies, respecting ILM settings
		loaded, err := m.loadPolicy(ilmFeature, ilmSupporter)
		if err != nil {
			return err
		}
		overwriteTemplate := templateFeature.overwrite || loaded

		// (3) load event type specific template respecting index lifecycle information
		if err := m.loadEventTemplate(templateFeature, ilmFeature, ilmSupporter, overwriteTemplate); err != nil {
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
	return newFeature(m.supporter.templateConfig.Enabled, m.supporter.templateConfig.Overwrite, true, loadMode)
}

func (m *manager) ilmFeature(loadMode libidxmgmt.LoadMode) feature {
	if m.supporter.st.ilmEnabled.Load() {
		f := newFeature(true, false, true, loadMode)
		if m.supporter.esIdxCfg.customized() {
			f.warn = msgIdxCfgIgnored
		}
		return f
	}

	var (
		warn      string
		err       error
		supported = true
	)
	if m.supporter.ilmConfig.Mode == libilm.ModeAuto {
		if m.supporter.esIdxCfg.customized() {
			warn = msgIlmDisabledCfg
		} else {
			warn = msgIlmDisabledES
			supported = false
		}
	} else if m.supporter.ilmConfig.Mode == libilm.ModeEnabled {
		err = errors.New(msgErrIlmDisabledES)
		supported = false
	}
	f := newFeature(false, false, supported, loadMode)
	f.warn, f.err = warn, err
	return f
}

func (m *manager) loadTemplate(templateFeature, ilmFeature feature) error {
	if !templateFeature.load {
		return nil
	}
	if ilmFeature.enabled || m.supporter.templateConfig.Name == "" && m.supporter.templateConfig.Pattern == "" {
		m.supporter.templateConfig.Name = fmt.Sprintf("%s-%s", apmPrefix, apmVersion)
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

func (m *manager) loadEventTemplate(templateFeature, ilmFeature feature, ilmSupporter libilm.Supporter, overwrite bool) error {
	if !templateFeature.load {
		return nil
	}
	templateCfg := ilm.Template(ilmFeature.enabled, templateFeature.enabled, overwrite,
		ilmSupporter.Alias().Name,
		ilmSupporter.Policy().Name)

	if err := m.clientHandler.Load(templateCfg, m.supporter.info, nil, m.supporter.migration); err != nil {
		return errors.Wrapf(err, "error loading template %+v", templateCfg.Name)
	}
	m.supporter.log.Infof("Finished template setup for %s.", templateCfg.Name)
	return nil
}

func (m *manager) loadPolicy(ilmFeature feature, ilmSupporter libilm.Supporter) (bool, error) {
	if !ilmFeature.load {
		return false, nil
	}
	policy := ilmSupporter.Policy().Name
	policyCreated, err := ilmSupporter.Manager(m.clientHandler).EnsurePolicy(ilmFeature.overwrite)
	if err != nil {
		return policyCreated, err
	}
	if !policyCreated {
		m.supporter.log.Infof("ILM policy %s exists already.", policy)
		return false, nil
	}
	m.supporter.log.Infof("ILM policy %s successfully loaded.", policy)
	return true, nil
}
func (m *manager) loadAlias(ilmFeature feature, ilmSupporter libilm.Supporter) error {
	if !ilmFeature.load {
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
