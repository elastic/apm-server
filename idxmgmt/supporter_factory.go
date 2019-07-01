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
	"errors"
	"fmt"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/idxmgmt"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/template"

	"github.com/elastic/apm-server/idxmgmt/ilm"
)

// functionality largely copied from libbeat

// MakeDefaultSupporter creates the index management supporter for APM that is passed to libbeat.
func MakeDefaultSupporter(log *logp.Logger, info beat.Info, configRoot *common.Config) (idxmgmt.Supporter, error) {

	const logName = "index-management"

	cfg := struct {
		ILMEnabled libilm.Mode            `config:"apm-server.ilm.enabled"`
		Template   *common.Config         `config:"setup.template"`
		Output     common.ConfigNamespace `config:"output"`
	}{}
	if configRoot != nil {
		if err := configRoot.Unpack(&cfg); err != nil {
			return nil, err
		}
	}

	ilmConfig := ilm.Config{Mode: cfg.ILMEnabled}

	tmplConfig, err := unpackTemplateConfig(cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("unpacking template config fails: %+v", err)
	}

	var esIdxCfg *esIndexConfig
	if cfg.Output.Name() == "elasticsearch" {
		if err := cfg.Output.Config().Unpack(&esIdxCfg); err != nil {
			return nil, fmt.Errorf("unpacking output elasticsearch index config fails: %+v", err)
		}
		if err := checkTemplateESSettings(tmplConfig, esIdxCfg); err != nil {
			return nil, err
		}
	}

	if log == nil {
		log = logp.NewLogger(logName)
	} else {
		log = log.Named(logName)
	}
	return newSupporter(log, info, tmplConfig, ilmConfig, esIdxCfg)
}

func checkTemplateESSettings(tmplCfg template.TemplateConfig, esIndexCfg *esIndexConfig) error {
	if !tmplCfg.Enabled {
		return nil
	}

	if esIndexCfg.Index != "" && (tmplCfg.Name == "" || tmplCfg.Pattern == "") {
		return errors.New("`setup.template.name` and `setup.template.pattern` have to be set if `output.elasticsearch` index name is modified")
	}
	return nil
}

func unpackTemplateConfig(cfg *common.Config) (template.TemplateConfig, error) {
	if cfg == nil {
		cfg = common.NewConfig()
	}

	config := template.DefaultConfig()
	err := cfg.Unpack(&config)
	return config, err
}
