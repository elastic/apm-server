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

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

// ProfilingConfig holds configuration related to profiling.
type ProfilingConfig struct {
	Enabled bool `config:"enabled"`

	ESConfig *elasticsearch.Config

	es *config.C
}

func (c *ProfilingConfig) Unpack(in *config.C) error {
	type profilingConfig ProfilingConfig
	cfg := profilingConfig(defaultProfilingConfig())
	if err := in.Unpack(&cfg); err != nil {
		return errors.Wrap(err, "error unpacking config")
	}
	cfg.Enabled = in.Enabled()
	*c = ProfilingConfig(cfg)

	if in.HasField("elasticsearch") {
		es, err := in.Child("elasticsearch", -1)
		if err != nil {
			return err
		}
		c.es = es
	}

	return errors.Wrap(c.Validate(), "invalid config")
}

func (c *ProfilingConfig) Validate() error {
	return nil
}

func (c *ProfilingConfig) setup(log *logp.Logger, outputESCfg *config.C) error {
	if !c.Enabled {
		return nil
	}
	// NOTE(axw) this behaviour below is a bit different to what we have
	// in other code that has Elasticsearch config. Here we always merge
	// the provided Elasticsearch config over the top of the Elasticsearch
	// output config. This allows one to provide only an API Key for
	// profiling (-E apm-server.profiling.elasticsearch.api_key=...), but
	// still use the same Elasticsearch output host, etc.
	if outputESCfg != nil {
		if err := outputESCfg.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "error unpacking output.elasticsearch config for profiling collection")
		}
	}
	if c.es != nil {
		if err := c.es.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "error unpacking apm-server.profiling.elasticsearch config for profiling collection")
		}
	}
	return nil
}

func defaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		Enabled:  false,
		ESConfig: elasticsearch.DefaultConfig(),
	}
}
