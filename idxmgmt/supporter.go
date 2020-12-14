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
	"go.uber.org/atomic"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/fmtstr"
	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/outil"
	"github.com/elastic/beats/v7/libbeat/template"

	"github.com/elastic/apm-server/datastreams"
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
	dataStreams        bool
	templateConfig     template.TemplateConfig
	ilmConfig          ilm.Config
	unmanagedIdxConfig unmanaged.Config
	migration          bool
	ilmSupporters      []libilm.Supporter

	st indexState
}

type indexState struct {
	ilmEnabled atomic.Bool
	isSet      atomic.Bool
}

// newDataStreamSelector returns an outil.Selector which routes events to
// a data stream based on well-defined data_stream.* fields in events.
func newDataStreamSelector() (outputs.IndexSelector, error) {
	fmtstr, err := fmtstr.CompileEvent(datastreams.IndexFormat)
	if err != nil {
		return nil, err
	}
	expr, err := outil.FmtSelectorExpr(fmtstr, "", outil.SelectorLowerCase)
	if err != nil {
		return nil, err
	}
	return outil.MakeSelector(expr), nil
}

type unmanagedIndexSelector outil.Selector

type ilmIndexSelector struct {
	unmanagedSel unmanagedIndexSelector
	ilmSel       outil.Selector
	st           *indexState
}

func newSupporter(log *logp.Logger, info beat.Info, cfg *IndexManagementConfig) (*supporter, error) {

	var (
		mode = cfg.ILM.Mode
		st   = indexState{}
	)

	var disableILM bool
	if cfg.Output.Name() != esKey || cfg.ILM.Mode == libilm.ModeDisabled {
		disableILM = true
	} else if cfg.ILM.Mode == libilm.ModeAuto {
		// ILM is set to "auto": disable if we're using data streams,
		// or if we're not using data streams but we're using customised,
		// unmanaged indices.
		if cfg.DataStreams || cfg.unmanagedIdxCfg.Customized() {
			disableILM = true
		}
	}
	if disableILM {
		mode = libilm.ModeDisabled
		st.isSet.CAS(false, true)
	}

	ilmSupporters, err := ilm.MakeDefaultSupporter(log, mode, cfg.ILM)
	if err != nil {
		return nil, err
	}

	return &supporter{
		log:                log,
		info:               info,
		dataStreams:        cfg.DataStreams,
		templateConfig:     cfg.Template,
		ilmConfig:          cfg.ILM,
		unmanagedIdxConfig: cfg.unmanagedIdxCfg,
		migration:          false,
		st:                 st,
		ilmSupporters:      ilmSupporters,
	}, nil
}

// Enabled indicates whether or not a callback should be registered to take care of setup.
// As long as ILM is enabled, this needs to return true, even if ilm.setup.enabled is set to false.
// The callback will not set up anything for ILM in that case, but signal the index selector that the setup is finished.
func (s *supporter) Enabled() bool {
	return s.templateConfig.Enabled || s.ilmConfig.Setup.Enabled || s.ilmConfig.Mode != libilm.ModeDisabled
}

// Manager instance takes only care of the setup.
// A clientHandler is passed in, which is required for figuring out the ILM state if set to `auto`.
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

// BuildSelector returns an index selector instance,
// depending on the supporter's config an ILM instance or an unmanaged index selector instance is returned.
// The ILM instance decides on every Select call whether or not to return ILM indices or regular ones.
func (s *supporter) BuildSelector(_ *common.Config) (outputs.IndexSelector, error) {
	if s.dataStreams {
		return newDataStreamSelector()
	}

	sel, err := s.buildSelector(s.unmanagedIdxConfig.SelectorConfig())
	if err != nil {
		return nil, err
	}
	unmanagedSel := unmanagedIndexSelector(sel)

	if s.st.isSet.Load() && !s.st.ilmEnabled.Load() {
		return unmanagedSel, nil
	}

	ilmSel, err := s.buildSelector(s.ilmConfig.SelectorConfig())
	if err != nil {
		return nil, err
	}

	return &ilmIndexSelector{
		unmanagedSel: unmanagedSel,
		ilmSel:       ilmSel,
		st:           &s.st,
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
		Case:             outil.SelectorLowerCase,
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
		if enabled, err := ilmSupporter.Manager(handler).CheckEnabled(); !enabled || err != nil {
			stSet()
			return
		}
	}

	s.st.ilmEnabled.CAS(false, true)
	stSet()
}

// Select either returns the index from the event's metadata or
// decides based on the supporter's ILM state whether or not an ILM index is returned
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
	return s.unmanagedSel.Select(evt)
}

// Select either returns the index from the event's metadata or
// the regular index.
func (s unmanagedIndexSelector) Select(evt *beat.Event) (string, error) {
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
