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
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/template"

	"github.com/elastic/apm-server/idxmgmt/ilm"
	"github.com/elastic/apm-server/idxmgmt/unmanaged"
	logs "github.com/elastic/apm-server/log"
)

type IndexManagementConfig struct {
	DataStreams bool
	Template    template.TemplateConfig
	ILM         ilm.Config
	Output      common.ConfigNamespace

	unmanagedIdxCfg unmanaged.Config
}

// MakeDefaultSupporter creates a new idxmgmt.Supporter, using the given root config.
//
// The Supporter will operate in one of three modes: data streams, legacy
// managed, and legacy unmanaged. The legacy modes exist purely to run
// apm-server without data streams or Fleet integration.
//
// If (Fleet) management is enabled, then no index, template, or ILM config
// should be set. Index (data stream) names will be well defined, based on
// the data type, service name, and user-defined namespace.
//
// If management is disabled, then the Supporter will operate in one of the
// legacy modes based on configuration.
func MakeDefaultSupporter(log *logp.Logger, info beat.Info, configRoot *common.Config) (idxmgmt.Supporter, error) {
	cfg, err := NewIndexManagementConfig(info, configRoot)
	if err != nil {
		return nil, err
	}
	log = namedLogger(log)
	return newSupporter(log, info, cfg)
}

func namedLogger(log *logp.Logger) *logp.Logger {
	if log == nil {
		return logp.NewLogger(logs.IndexManagement)
	}
	return log.Named(logs.IndexManagement)
}

// NewIndexManagementConfig extracts and validates index management config from info and configRoot.
func NewIndexManagementConfig(info beat.Info, configRoot *common.Config) (*IndexManagementConfig, error) {
	var cfg struct {
		DataStreams            *common.Config         `config:"apm-server.data_streams"`
		RegisterIngestPipeline *common.Config         `config:"apm-server.register.ingest.pipeline"`
		ILM                    *common.Config         `config:"apm-server.ilm"`
		Template               *common.Config         `config:"setup.template"`
		Output                 common.ConfigNamespace `config:"output"`
	}
	if configRoot != nil {
		if err := configRoot.Unpack(&cfg); err != nil {
			return nil, err
		}
	}

	templateConfig, err := unpackTemplateConfig(cfg.Template)
	if err != nil {
		return nil, errors.Wrap(err, "unpacking template config failed")
	}

	ilmConfig, err := ilm.NewConfig(info, cfg.ILM)
	if err != nil {
		return nil, errors.Wrap(err, "creating ILM config fails")
	}

	var unmanagedIdxCfg unmanaged.Config
	if cfg.Output.Name() == esKey {
		if err := cfg.Output.Config().Unpack(&unmanagedIdxCfg); err != nil {
			return nil, errors.Wrap(err, "failed to unpack output.elasticsearch config")
		}
		if err := checkTemplateESSettings(templateConfig, &unmanagedIdxCfg); err != nil {
			return nil, err
		}
	}

	return &IndexManagementConfig{
		Output:   cfg.Output,
		Template: templateConfig,
		ILM:      ilmConfig,

		unmanagedIdxCfg: unmanagedIdxCfg,
	}, nil
}

func checkTemplateESSettings(tmplCfg template.TemplateConfig, indexCfg *unmanaged.Config) error {
	if !tmplCfg.Enabled || indexCfg == nil {
		return nil
	}
	if indexCfg.Index != "" && (tmplCfg.Name == "" || tmplCfg.Pattern == "") {
		return errors.New("`setup.template.name` and `setup.template.pattern` have to be set if `output.elasticsearch` index name is modified")
	}
	return nil
}

// unpackTemplateConfig merges APM-specific template settings with (possibly nil)
// user-defined config, unpacks it over template.DefaultConfig(), returning the result.
func unpackTemplateConfig(userTemplateConfig *common.Config) (template.TemplateConfig, error) {
	templateConfig := common.MustNewConfigFrom(`
settings:
  index:
    codec: best_compression
    mapping.total_fields.limit: 2000
    number_of_shards: 1
  _source.enabled: true`)
	if userTemplateConfig != nil {
		if err := templateConfig.Merge(userTemplateConfig); err != nil {
			return template.TemplateConfig{}, errors.Wrap(err, "merging failed")
		}
	}
	out := template.DefaultConfig()
	if err := templateConfig.Unpack(&out); err != nil {
		return template.TemplateConfig{}, err
	}
	return out, nil
}
