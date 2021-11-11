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

	"go.uber.org/atomic"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/outil"
	"github.com/elastic/beats/v7/libbeat/template"

	"github.com/elastic/apm-server/idxmgmt/ilm"
	"github.com/elastic/apm-server/idxmgmt/unmanaged"
)

// The index management supporter holds information around ES template, ILM strategy and index setup for Elasticsearch.
// The supporter methods are called from within libbeat code during setup time and on startup.
// The supporter takes care of template loading respecting the ILM strategy, loading ILM policies and write aliases.
// The supporter also ensures the default index and indices settings are used, if not overwritten in the config by the user.
//
// Functionality is partly copied from libbeat.

const esKey = "elasticsearch"

type supporter struct {
	log                *logp.Logger
	info               beat.Info
	templateConfig     template.TemplateConfig
	ilmConfig          ilm.Config
	outputConfig       common.ConfigNamespace
	unmanagedIdxConfig unmanaged.Config
	ilmEnabled         atomic.Bool
}

type indexSelector outil.Selector

func newSupporter(log *logp.Logger, info beat.Info, cfg *IndexManagementConfig) (*supporter, error) {
	return &supporter{
		log:                log,
		info:               info,
		templateConfig:     cfg.Template,
		ilmConfig:          cfg.ILM,
		outputConfig:       cfg.Output,
		unmanagedIdxConfig: cfg.unmanagedIdxCfg,
	}, nil
}

// Enabled indicates whether or not a callback should be registered to take care of setup.
// As long as ILM is enabled, this needs to return true, even if ilm.setup.enabled is set to false.
// The callback will not set up anything for ILM in that case, but signal the index selector that the setup is finished.
func (s *supporter) Enabled() bool {
	return s.templateConfig.Enabled || s.ilmConfig.Setup.Enabled || s.ilmConfig.Enabled
}

// Manager instance takes only care of the setup.
// A clientHandler is passed in, which is required for figuring out the ILM state if set to `auto`.
func (s *supporter) Manager(clientHandler libidxmgmt.ClientHandler, assets libidxmgmt.Asseter) libidxmgmt.Manager {
	return &manager{
		supporter:     s,
		clientHandler: clientHandler,
		assets:        assets,
	}
}

// BuildSelector returns an index selector instance,
// depending on the supporter's config an ILM instance or an unmanaged index selector instance is returned.
// The ILM instance decides on every Select call whether or not to return ILM indices or regular ones.
func (s *supporter) BuildSelector(_ *common.Config) (outputs.IndexSelector, error) {
	unmanagedSelector, err := s.buildSelector(s.unmanagedIdxConfig.SelectorConfig())
	if err != nil {
		return nil, err
	}
	ilmSelector, err := s.buildSelector(s.ilmConfig.SelectorConfig())
	if err != nil {
		return nil, err
	}

	if !s.ilmConfig.Enabled {
		return indexSelector(unmanagedSelector), nil
	}
	return indexSelector(ilmSelector), nil
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
		Case:             outil.SelectorLowerCase,
	}
	return outil.BuildSelectorFromConfig(cfg, buildSettings)
}

// Select either returns the index from the event's metadata or the regular index.
func (s indexSelector) Select(evt *beat.Event) (string, error) {
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
			return strings.ToLower(alias)
		}
	}

	// returns index from meta + day
	if tmp := evt.Meta["index"]; tmp != nil {
		if idx, ok := tmp.(string); ok {
			ts := evt.Timestamp.UTC()
			return fmt.Sprintf("%s-%d.%02d.%02d",
				strings.ToLower(idx), ts.Year(), ts.Month(), ts.Day())
		}
	}

	return ""
}
