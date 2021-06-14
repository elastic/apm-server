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

package config

import (
	"errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/kibana"
)

func NewIntegrationConfig(rootConfig *common.Config) (*IntegrationConfig, error) {
	config := &IntegrationConfig{
		DataStream: &DataStream{
			Namespace: "default",
		},
	}
	if err := rootConfig.Unpack(config); err != nil {
		return nil, err
	}
	if config.APMServer == nil {
		return nil, errors.New("'apm-server' not found in integration config")
	}
	return config, nil
}

// IntegrationConfig that comes from Elastic Agent
type IntegrationConfig struct {
	ID         string         `config:"id"`
	Name       string         `config:"name"`
	Revision   int            `config:"revision"`
	Type       string         `config:"type"`
	UseOutput  string         `config:"use_output"`
	Meta       *Meta          `config:"meta"`
	DataStream *DataStream    `config:"data_stream"`
	APMServer  *common.Config `config:"apm-server"`
	Fleet      Fleet          `config:"fleet"`
}

type DataStream struct {
	Namespace string `config:"namespace"`
}

type Meta struct {
	Package *Package `config:"package"`
}

type Package struct {
	Name    string `config:"name"`
	Version string `config:"version"`
}

type Fleet struct {
	Kibana       kibana.ClientConfig `config:"kibana"`
	Hosts        []string            `config:"hosts"`
	Protocol     string              `config:"protocol"`
	AccessAPIKey string              `config:"access_api_key"`
	TLS          *tlscommon.Config   `config:"ssl"`
}
