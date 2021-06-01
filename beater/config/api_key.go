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
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/elasticsearch"
)

const apiKeyLimit = 100

// APIKeyConfig can be used for authorizing against the APM Server via API Keys.
type APIKeyConfig struct {
	Enabled     bool                  `config:"enabled"`
	LimitPerMin int                   `config:"limit"`
	ESConfig    *elasticsearch.Config `config:"elasticsearch"`

	esConfigured bool
}

func (c *APIKeyConfig) setup(log *logp.Logger, outputESCfg *common.Config) error {
	if !c.Enabled || c.esConfigured || outputESCfg == nil {
		return nil
	}
	log.Info("Falling back to elasticsearch output for API Key usage")
	if err := outputESCfg.Unpack(c.ESConfig); err != nil {
		return errors.Wrap(err, "unpacking Elasticsearch config into API key config")
	}
	return nil
}

func defaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{Enabled: false, LimitPerMin: apiKeyLimit, ESConfig: elasticsearch.DefaultConfig()}
}

func (c *APIKeyConfig) Unpack(inp *common.Config) error {
	type underlyingAPIKeyConfig APIKeyConfig
	if err := inp.Unpack((*underlyingAPIKeyConfig)(c)); err != nil {
		return errors.Wrap(err, "error unpacking api_key config")
	}
	c.esConfigured = inp.HasField("elasticsearch")
	return nil
}
