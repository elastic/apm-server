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
	"go.uber.org/atomic"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	"github.com/elastic/beats/libbeat/outputs/outil"
	"github.com/elastic/beats/libbeat/template"

	"github.com/elastic/apm-server/idxmgmt/ilm"
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

const esKey = "elasticsearch"

type supporter struct {
	log            *logp.Logger
	info           beat.Info
	templateConfig template.TemplateConfig
	ilmConfig      ilm.Config
	esIdxCfg       *esIndexConfig
	migration      bool
	ilmSupporters  []libilm.Supporter

	st indexState
}

type indexState struct {
	ilmEnabled atomic.Bool
	isSet      atomic.Bool
}

type defaultIndexSelector outil.Selector

type ilmIndexSelector struct {
	defaultSel defaultIndexSelector
	ilmSel     outil.Selector
	st         *indexState
}

func newSupporter(
	log *logp.Logger,
	info beat.Info,
	templateConfig template.TemplateConfig,
	ilmConfig ilm.Config,
	outConfig common.ConfigNamespace,
) (*supporter, error) {

	var (
		esIdxCfg *esIndexConfig
		mode     = ilmConfig.Mode
		st       = indexState{}
	)

	if outConfig.Name() == esKey {
		if err := outConfig.Config().Unpack(&esIdxCfg); err != nil {
			return nil, fmt.Errorf("unpacking output elasticsearch index config fails: %+v", err)
		}

		if err := checkTemplateESSettings(templateConfig, esIdxCfg); err != nil {
			return nil, err
		}
	}

	if outConfig.Name() != esKey ||
		ilmConfig.Mode == libilm.ModeDisabled ||
		ilmConfig.Mode == libilm.ModeAuto && esIdxCfg.customized() {

		mode = libilm.ModeDisabled
		st.isSet.CAS(false, true)
	}

	ilmSupporters, err := ilmSupporters(log, mode, info)
	if err != nil {
		return nil, err
	}

	return &supporter{
		log:            log,
		info:           info,
		templateConfig: templateConfig,
		ilmConfig:      ilmConfig,
		esIdxCfg:       esIdxCfg,
		migration:      false,
		st:             st,
		ilmSupporters:  ilmSupporters,
	}, nil
}

func (s *supporter) Enabled() bool {
	return s.templateConfig.Enabled || s.ilmConfig.Enabled()
}

func (s *supporter) Manager(
	clientHandler libidxmgmt.ClientHandler,
	assets libidxmgmt.Asseter,
) libidxmgmt.Manager {
	s.setIlmState(clientHandler)
	return &manager{
		supporter:     s,
		clientHandler: clientHandler,
		assets:        assets,
	}
}

func (s *supporter) BuildSelector(cfg *common.Config) (outputs.IndexSelector, error) {
	sel, err := s.buildSelector(indices(cfg))
	if err != nil {
		return nil, err
	}
	ordIdxSel := defaultIndexSelector(sel)

	if s.st.isSet.Load() && !s.st.ilmEnabled.Load() {
		return ordIdxSel, nil
	}

	ilmSel, err := s.buildSelector(ilmIndices(cfg))
	if err != nil {
		return nil, err
	}

	return &ilmIndexSelector{
		defaultSel: ordIdxSel,
		ilmSel:     ilmSel,
		st:         &s.st,
	}, nil
}

func (s *supporter) buildSelector(cfg *common.Config, err error) (outil.Selector, error) {
	if err != nil {
		return outil.Selector{}, err
	}

	buildSettings := outil.Settings{
		Key:              "index",
		MultiKey:         "indices",
		EnableSingleOnly: true,
		FailEmpty:        true,
	}
	return outil.BuildSelectorFromConfig(cfg, buildSettings)
}

func (s *supporter) setIlmState(handler libidxmgmt.ClientHandler) {
	stSet := func() { s.st.isSet.CAS(false, true) }

	if s.st.isSet.Load() {
		return
	}
	if s.st.ilmEnabled.Load() {
		stSet()
		return
	}

	for _, ilmSupporter := range s.ilmSupporters {
		if enabled, err := ilmSupporter.Manager(handler).Enabled(); !enabled || err != nil {
			stSet()
			return
		}
	}

	s.st.ilmEnabled.CAS(false, true)
	stSet()
}

func (s *ilmIndexSelector) Select(evt *beat.Event) (string, error) {
	if idx := getEventCustomIndex(evt); idx != "" {
		return idx, nil
	}
	if !s.st.isSet.Load() {
		return "", errors.New("setup not finished")
	}

	if s.st.ilmEnabled.Load() {
		return s.ilmSel.Select(evt)
	}
	return s.defaultSel.Select(evt)
}

func (s defaultIndexSelector) Select(evt *beat.Event) (string, error) {
	if idx := getEventCustomIndex(evt); idx != "" {
		return idx, nil
	}
	return outil.Selector(s).Select(evt)
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

func checkTemplateESSettings(tmplCfg template.TemplateConfig, esIndexCfg *esIndexConfig) error {
	if !tmplCfg.Enabled || esIndexCfg == nil {
		return nil
	}

	if esIndexCfg.Index != "" && (tmplCfg.Name == "" || tmplCfg.Pattern == "") {
		return errors.New("`setup.template.name` and `setup.template.pattern` have to be set if `output.elasticsearch` index name is modified")
	}
	return nil
}

func ilmSupporters(log *logp.Logger, mode libilm.Mode, info beat.Info) ([]libilm.Supporter, error) {
	var (
		ilmSupporters []libilm.Supporter
		err           error
		ilmCfg        *common.Config
	)

	for event, index := range eventIdxNames(false) {
		if ilmCfg, err = common.NewConfigFrom(common.MapStr{
			"enabled":     ilm.ModeString(mode),
			"event":       event,
			"policy_name": idxStr(event, ""),
			"alias_name":  index},
		); err != nil {
			return nil, errors.Wrapf(err, "error creating index-management config")
		}
		ilmSupporter, err := ilm.MakeDefaultSupporter(log, info, ilmCfg)
		if err != nil {
			return nil, err
		}
		ilmSupporters = append(ilmSupporters, ilmSupporter)
	}
	return ilmSupporters, nil
}
