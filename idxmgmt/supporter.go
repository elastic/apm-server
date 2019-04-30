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

	"github.com/elastic/apm-server/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/idxmgmt"
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
	log          *logp.Logger
	ilmSupporter libilm.Supporter
	info         beat.Info
	migration    bool
	templateCfg  template.TemplateConfig
}

type manager struct {
	supporter     *supporter
	ilm           libilm.Manager
	clientHandler idxmgmt.ClientHandler
	assets        idxmgmt.Asseter
}

type selector outil.Selector

func newSupporter(
	log *logp.Logger,
	info beat.Info,
	tmplConfig template.TemplateConfig,
	ilmConfig *common.Config,
	migration bool,
) (*supporter, error) {

	// creates a Noop ILM handler for now.
	ilmSupporter, err := ilm.MakeDefaultSupporter(log, info, ilmConfig)
	if err != nil {
		return nil, err
	}

	return &supporter{
		log:          log,
		ilmSupporter: ilmSupporter,
		info:         info,
		templateCfg:  tmplConfig,
		migration:    migration,
	}, nil
}

func (s *supporter) Enabled() bool {
	return s.templateCfg.Enabled
}

func (s *supporter) ILM() libilm.Supporter {
	return s.ilmSupporter
}

func (s *supporter) TemplateConfig(_ bool) (template.TemplateConfig, error) {
	return s.templateCfg, nil
}

func (s *supporter) Manager(
	clientHandler idxmgmt.ClientHandler,
	assets idxmgmt.Asseter,
) idxmgmt.Manager {
	ilm := s.ilmSupporter.Manager(clientHandler)

	return &manager{
		supporter:     s,
		ilm:           ilm,
		clientHandler: clientHandler,
		assets:        assets,
	}
}

func (s *supporter) BuildSelector(cfg *common.Config) (outputs.IndexSelector, error) {
	var err error
	var selCfg = common.NewConfig()

	// set defaults
	if !cfg.HasField("index") {
		// set fallback default index
		selCfg.SetString("index", -1, defaultIndex)

		// set default indices if not set
		if !cfg.HasField("indices") {
			if indicesCfg, err := common.NewConfigFrom(defaultIndices); err == nil {
				selCfg.SetChild("indices", -1, indicesCfg)
			}
		}
	}

	// use custom config settings where available
	if cfg.HasField("index") {
		indexName, err := cfg.String("index", -1)
		if err != nil {
			return nil, err
		}
		selCfg.SetString("index", -1, indexName)
	}
	if cfg.HasField("indices") {
		sub, err := cfg.Child("indices", -1)
		if err != nil {
			return nil, err
		}
		selCfg.SetChild("indices", -1, sub)
	}

	buildSettings := outil.Settings{
		Key:              "index",
		MultiKey:         "indices",
		EnableSingleOnly: true,
		FailEmpty:        true,
	}

	indexSel, err := outil.BuildSelectorFromConfig(selCfg, buildSettings)
	if err != nil {
		return nil, err
	}

	return selector(indexSel), nil
}

func (s selector) Select(evt *beat.Event) (string, error) {
	if idx := getEventCustomIndex(evt); idx != "" {
		return idx, nil
	}
	return outil.Selector(s).Select(evt)
}

func (m *manager) Setup(templateMode, ilmMode idxmgmt.LoadMode) error {
	return m.load(templateMode, ilmMode)
}

func (m *manager) Load() error {
	return m.load(idxmgmt.LoadModeUnset, idxmgmt.LoadModeUnset)
}

func (m *manager) load(templateMode, _ idxmgmt.LoadMode) error {
	log := m.supporter.log

	// create and install template
	if m.supporter.templateCfg.Enabled {
		tmplCfg := m.supporter.templateCfg

		if templateMode == idxmgmt.LoadModeForce {
			tmplCfg.Overwrite = true
		}

		fields := m.assets.Fields(m.supporter.info.Beat)
		err := m.clientHandler.Load(tmplCfg, m.supporter.info, fields, m.supporter.migration)
		if err != nil {
			return fmt.Errorf("error loading Elasticsearch template: %+v", err)
		}

		log.Info("Loaded index template.")
	}

	return nil
}

func getEventCustomIndex(evt *beat.Event) string {
	if len(evt.Meta) == 0 {
		return ""
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
