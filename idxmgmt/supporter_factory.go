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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/template"

	"github.com/elastic/apm-server/idxmgmt/ilm"
	logs "github.com/elastic/apm-server/log"
)

// functionality largely copied from libbeat

type IndexManagementConfig struct {
	DataStreams bool
	Template    template.TemplateConfig
	ILM         ilm.Config
	Output      common.ConfigNamespace
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
	if log == nil {
		log = logp.NewLogger(logs.IndexManagement)
	} else {
		log = log.Named(logs.IndexManagement)
	}
	return newSupporter(log, info, cfg)
}

func NewIndexManagementConfig(info beat.Info, configRoot *common.Config) (*IndexManagementConfig, error) {
	cfg := struct {
		DataStreams *common.Config         `config:"apm-server.data_streams"`
		ILM         *common.Config         `config:"apm-server.ilm"`
		Template    *common.Config         `config:"setup.template"`
		Output      common.ConfigNamespace `config:"output"`
	}{}
	if configRoot != nil {
		if err := configRoot.Unpack(&cfg); err != nil {
			return nil, err
		}
	}

	tmplConfig, err := unpackTemplateConfig(cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("unpacking template config fails: %+v", err)
	}

	ilmConfig, err := ilm.NewConfig(info, cfg.ILM)
	if err != nil {
		return nil, fmt.Errorf("creating ILM config fails: %v", err)
	}

	return &IndexManagementConfig{
		DataStreams: cfg.DataStreams.Enabled(),
		Template:    tmplConfig,
		ILM:         ilmConfig,
		Output:      cfg.Output,
	}, nil
}

func unpackTemplateConfig(cfg *common.Config) (template.TemplateConfig, error) {
	config := template.DefaultConfig()
	if cfg == nil {
		return config, nil
	}
	err := cfg.Unpack(&config)
	return config, err
}
