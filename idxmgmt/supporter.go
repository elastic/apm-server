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
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	"github.com/elastic/beats/libbeat/outputs/outil"
	"github.com/elastic/beats/libbeat/template"
)

// The index management supporter holds information around ES template, ILM strategy and index setup for Elasticsearch.
// The supporter methods are called from within libbeat code during setup time and on startup.
// The supporter takes care of template loading respecting the ILM strategy. Currently APM does not support setting up ILM
// from within the server, so it uses the ILM noop supporter.
// The supporter also ensures the default index and indices settings are used, if not overwritten in the config by the user.
//
// Functionality is largely copied from libbeat, and mainly differs in
// - the used ILM supporter
// - ignoring ILM in template setup for now
// - the default index and indices that are set.
//

type supporter struct {
	log            *logp.Logger
	info           beat.Info
	templateConfig template.TemplateConfig
	ilmConfig      ilm.Config
	migration      bool
}

type manager struct {
	supporter     *supporter
	clientHandler libidxmgmt.ClientHandler
	assets        libidxmgmt.Asseter
}

type selector outil.Selector

type component struct {
	enabled, overwrite, load bool
}

func newComponent(enabled, overwrite bool, mode libidxmgmt.LoadMode) component {
	if mode == libidxmgmt.LoadModeUnset {
		mode = libidxmgmt.LoadModeDisabled
	}
	if mode >= libidxmgmt.LoadModeOverwrite {
		overwrite = true
	}
	if mode == libidxmgmt.LoadModeForce {
		enabled = true
	}
	load := mode.Enabled() && enabled
	return component{enabled: enabled, overwrite: overwrite, load: load}
}

func newSupporter(
	log *logp.Logger,
	info beat.Info,
	templateConfig template.TemplateConfig,
	ilmConfig ilm.Config,
) (*supporter, error) {
	return &supporter{
		log:            log,
		info:           info,
		templateConfig: templateConfig,
		ilmConfig:      ilmConfig,
		migration:      false,
	}, nil
}

func (s *supporter) Enabled() bool {
	return s.templateConfig.Enabled || s.ilmConfig.Enabled
}

func (s *supporter) Manager(
	clientHandler libidxmgmt.ClientHandler,
	assets libidxmgmt.Asseter,
) libidxmgmt.Manager {
	return &manager{
		supporter:     s,
		clientHandler: clientHandler,
		assets:        assets,
	}
}

func (s *supporter) BuildSelector(cfg *common.Config) (outputs.IndexSelector, error) {
	if s.ilmConfig.Enabled {
		idcs, err := ilmIndices(cfg)
		return s.buildSelector(idcs, err)
	}
	return s.buildSelector(indices(cfg))
}

func (s selector) Select(evt *beat.Event) (string, error) {
	if idx := getEventCustomIndex(evt); idx != "" {
		return idx, nil
	}
	return outil.Selector(s).Select(evt)
}

func (s *supporter) buildSelector(cfg *common.Config, err error) (outputs.IndexSelector, error) {
	if err != nil {
		return nil, err
	}

	buildSettings := outil.Settings{
		Key:              "index",
		MultiKey:         "indices",
		EnableSingleOnly: true,
		FailEmpty:        true,
	}
	indexSel, err := outil.BuildSelectorFromConfig(cfg, buildSettings)
	if err != nil {
		return nil, err
	}

	return selector(indexSel), nil
}

func (m *manager) VerifySetup(loadTemplate, loadILM libidxmgmt.LoadMode) (bool, string) {
	ilmComponent := newComponent(m.supporter.ilmConfig.Enabled, false, loadILM)
	templateComponent := newComponent(m.supporter.templateConfig.Enabled,
		m.supporter.templateConfig.Overwrite, loadTemplate)

	if ilmComponent.load && !templateComponent.load {
		return false, "Loading ILM policy and write alias without loading template " +
			"is not recommended. Check your configuration."
	}
	if templateComponent.load && !ilmComponent.load && ilmComponent.enabled {
		return false, "Loading template with ILM settings whithout loading ILM policy and alias can lead " +
			"to issues and is not recommended. Check your configuration"
	}
	var warn string
	if !ilmComponent.load {
		warn += "ILM policy and write alias loading not enabled. "
	}
	if !templateComponent.load {
		warn += "Template loading not enabled."
	}
	return warn == "", warn
}

func (m *manager) Setup(loadTemplate, loadILM libidxmgmt.LoadMode) error {
	log := m.supporter.log

	ilmComponent := newComponent(m.supporter.ilmConfig.Enabled, false, loadILM)
	templateComponent := newComponent(m.supporter.templateConfig.Enabled,
		m.supporter.templateConfig.Overwrite, loadTemplate)
	m.supporter.templateConfig.Enabled = templateComponent.enabled
	m.supporter.templateConfig.Overwrite = templateComponent.overwrite

	//setup index management:
	//(1) load general apm template
	//(2) load policy per event type
	//(3) create template per event respecting lifecycle settings
	//(4) load write alias per event type AFTER the template has been created,
	//    as this step also automatically creates an index, it is important the matching templates are already there

	var (
		ilmSupporter      libilm.Supporter
		err               error
		ilmCfg            *common.Config
		policyCreated     bool
		overwriteTemplate = templateComponent.overwrite
		templateCfg       template.TemplateConfig
	)

	//(1) load general apm template
	//only set to user configured name and pattern if ilm is disabled
	//default template name and pattern, must be the same whether or not ilm is enabled or not,
	//allowing former templates to be overwritten
	if templateComponent.load {
		if ilmComponent.enabled || m.supporter.templateConfig.Name == "" && m.supporter.templateConfig.Pattern == "" {
			m.supporter.templateConfig.Name = fmt.Sprintf("%s-%s", apmPrefix, apmVersion)
			m.supporter.log.Infof("Set setup.template.name to '%s'.", m.supporter.templateConfig.Name)
			m.supporter.templateConfig.Pattern = m.supporter.templateConfig.Name + "*"
			m.supporter.log.Infof("Set setup.template.pattern to '%s'.", m.supporter.templateConfig.Pattern)
		}
		if err := m.clientHandler.Load(m.supporter.templateConfig, m.supporter.info,
			m.assets.Fields(m.supporter.info.Beat), m.supporter.migration); err != nil {
			return fmt.Errorf("error loading Elasticsearch template: %+v", err)
		}
		log.Infof("Finished loading index template.")
	}

	for event, index := range eventIdxNames(false) {
		if ilmCfg, err = common.NewConfigFrom(common.MapStr{
			"enabled":     ilmComponent.enabled,
			"event":       event,
			"policy_name": idxStr(event, ""),
			"alias_name":  index},
		); err != nil {
			return errors.Wrapf(err, "error creating index-management config")
		}
		if ilmSupporter, err = ilm.MakeDefaultSupporter(log, m.supporter.info, ilmCfg); err != nil {
			return err
		}
		ilmManager := ilmSupporter.Manager(m.clientHandler)
		policy := ilmSupporter.Policy().Name
		alias := ilmSupporter.Alias().Name

		if ilmComponent.load {
			//(2) load event type policies, respecting ILM settings
			if policyCreated, err = ilmManager.EnsurePolicy(ilmComponent.overwrite); err != nil {
				return err
			}
			if policyCreated {
				log.Infof("ILM policy %s successfully loaded.", policy)
				overwriteTemplate = true
			} else {
				log.Infof("ILM policy %s exists already.", policy)
			}
		}

		// (3) load event type specific template respecting index lifecycle information
		if templateComponent.load {
			templateCfg = ilm.Template(ilmComponent.enabled, templateComponent.enabled, overwriteTemplate, alias, policy)
			if err = m.clientHandler.Load(templateCfg, m.supporter.info, nil, m.supporter.migration); err != nil {
				return errors.Wrapf(err, "error loading template %+v", templateCfg.Name)
			}
			log.Infof("Finished template setup for %s.", templateCfg.Name)
		}

		if ilmComponent.load {
			//(4) load ilm write aliases
			//    ensure write aliases are created AFTER template creation
			if err = ilmManager.EnsureAlias(); err != nil {
				if libilm.ErrReason(err) != libilm.ErrAliasAlreadyExists {
					return err
				}
				log.Infof("Write alias %s exists already.", alias)
			} else {
				log.Infof("Write alias %s successfully generated.", alias)
			}
		}
	}

	log.Info("Finished index management setup.")
	return nil
}

// this logic is copied and aligned with handling in beats.
func getEventCustomIndex(evt *beat.Event) string {
	if len(evt.Meta) == 0 {
		return ""
	}

	// returns index from alias
	if tmp := evt.Meta["alias"]; tmp != nil {
		if alias, ok := tmp.(string); ok {
			return alias
		}
	}

	// returns index from meta + day
	if tmp := evt.Meta["index"]; tmp != nil {
		if idx, ok := tmp.(string); ok {
			ts := evt.Timestamp.UTC()
			return fmt.Sprintf("%s-%d.%02d.%02d",
				idx, ts.Year(), ts.Month(), ts.Day())
		}
	}

	return ""
}
