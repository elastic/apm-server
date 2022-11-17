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
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-ucfg"
)

// ProfilingConfig holds configuration related to profiling.
type ProfilingConfig struct {
	Enabled bool `config:"enabled"`

	// ESConfig holds Elasticsearch configuration for writing
	// profiling stacktrace events and metadata documents.
	ESConfig *elasticsearch.Config

	// MetricsESConfig holds Elasticsearch configuration for
	// writing profiling host agent metric documents.
	MetricsESConfig *elasticsearch.Config

	// ILMConfig
	ILMConfig *ProfilingILMConfig `config:"keyvalue_retention"`

	es        *config.C
	metricsES *config.C
	ilm       *config.C
}

func (c *ProfilingConfig) Unpack(in *config.C) error {
	type profilingConfig ProfilingConfig
	cfg := profilingConfig(defaultProfilingConfig())
	if err := in.Unpack(&cfg); err != nil {
		return errors.Wrap(err, "error unpacking config")
	}
	cfg.Enabled = in.Enabled()
	*c = ProfilingConfig(cfg)

	es, err := in.Child("elasticsearch", -1)
	if err == nil {
		c.es = es
	} else {
		var ucfgError ucfg.Error
		if !errors.As(err, &ucfgError) || ucfgError.Reason() != ucfg.ErrMissing {
			return err
		}
	}

	es, err = in.Child("metrics.elasticsearch", -1)
	if err == nil {
		c.metricsES = es
	} else {
		var ucfgError ucfg.Error
		if !errors.As(err, &ucfgError) || ucfgError.Reason() != ucfg.ErrMissing {
			return err
		}
	}

	if ilm, err := in.Child("keyvalue_retention", -1); err == nil {
		c.ilm = ilm
	} else {
		var ucfgError ucfg.Error
		if !errors.As(err, &ucfgError) || ucfgError.Reason() != ucfg.ErrMissing {
			return err
		}
	}

	return errors.Wrap(c.Validate(), "invalid config")
}

func (c *ProfilingConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.metricsES == nil {
		return errors.New("missing required field 'apm-server.profiling.metrics.elasticsearch'")
	}
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
			return errors.Wrap(err, "error unpacking output.elasticsearch config for profiling event collection")
		}
		// NOTE(axw) we intentionally do not unpack `output.elasticsearch`
		// into `apm-server.profiling.metrics.elasticsearch`, as host agent
		// metrics are expected to go to a separate cluster.
	}
	if c.es != nil {
		if err := c.es.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "error unpacking apm-server.profiling.elasticsearch config for profiling collection")
		}
	}
	if c.metricsES != nil {
		if err := c.metricsES.Unpack(&c.MetricsESConfig); err != nil {
			return errors.Wrap(err, "error unpacking apm-server.profiling.metrics.elasticsearch config for profiling host agent metrics collection")
		}
	}
	if c.ilm != nil {
		if err := c.ilm.Unpack(&c.ILMConfig); err != nil {
			return errors.Wrap(err, "error unpacking apm-server.profiling.keyvalue_retention config for profiling K/V data retention")
		}
	}
	return nil
}

type ProfilingILMConfig struct {
	Age         time.Duration `config:"age"`
	SizeInBytes uint64        `config:"size_bytes"`
	Interval    time.Duration `config:"execution_interval"`
}

func defaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		Enabled:         false,
		ESConfig:        elasticsearch.DefaultConfig(),
		MetricsESConfig: elasticsearch.DefaultConfig(),
		ILMConfig:       defaultProfilingILMConfig(),
	}
}

func defaultProfilingILMConfig() *ProfilingILMConfig {
	return &ProfilingILMConfig{
		// 2 months
		Age: 1440 * time.Hour,
		// 100 GiB
		SizeInBytes: 107374182400,
		Interval:    12 * time.Hour,
	}
}
