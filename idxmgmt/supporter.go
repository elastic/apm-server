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
	libidxmgmt "github.com/elastic/beats/libbeat/idxmgmt"
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
	esIdxCfg       *esIndexConfig
	migration      bool
}

type selector outil.Selector

func newSupporter(
	log *logp.Logger,
	info beat.Info,
	templateConfig template.TemplateConfig,
	ilmConfig ilm.Config,
	esOutputConfig *esIndexConfig,
) (*supporter, error) {
	return &supporter{
		log:            log,
		info:           info,
		templateConfig: templateConfig,
		ilmConfig:      ilmConfig,
		esIdxCfg:       esOutputConfig,
		migration:      false,
	}, nil
}

func (s *supporter) Enabled() bool {
	return s.templateConfig.Enabled || s.ilmConfig.Enabled()
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
	if s.ilmConfig.Enabled() {
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
