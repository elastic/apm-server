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

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// SamplingConfig holds configuration related to sampling.
type SamplingConfig struct {
	// KeepUnsampled controls whether unsampled
	// transactions should be recorded.
	KeepUnsampled bool `config:"keep_unsampled"`

	// Tail holds tail-sampling configuration.
	Tail *TailSamplingConfig `config:"tail"`
}

// TailSamplingConfig holds configuration related to tail-sampling.
type TailSamplingConfig struct {
	Enabled bool `config:"enabled"`

	DefaultSampleRate     float64               `config:"default_sample_rate" validate:"min=0, max=1"`
	ESConfig              *elasticsearch.Config `config:"elasticsearch"`
	Interval              time.Duration         `config:"interval" validate:"min=1s"`
	IngestRateDecayFactor float64               `config:"ingest_rate_decay" validate:"min=0, max=1"`
	StorageDir            string                `config:"storage_dir"`
	StorageGCInterval     time.Duration         `config:"storage_gc_interval" validate:"min=1s"`
	TTL                   time.Duration         `config:"ttl" validate:"min=1s"`

	esConfigured bool
}

func (c *TailSamplingConfig) Unpack(in *common.Config) error {
	type tailSamplingConfig TailSamplingConfig
	cfg := tailSamplingConfig(defaultTailSamplingConfig())
	if err := in.Unpack(&cfg); err != nil {
		return errors.Wrap(err, "error unpacking tail sampling config")
	}
	*c = TailSamplingConfig(cfg)
	c.esConfigured = in.HasField("elasticsearch")
	return nil
}

func (c *TailSamplingConfig) setup(log *logp.Logger, outputESCfg *common.Config) error {
	if !c.Enabled {
		return nil
	}
	if !c.esConfigured && outputESCfg != nil {
		log.Info("Falling back to elasticsearch output for tail-sampling")
		if err := outputESCfg.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "error unpacking output.elasticsearch config for tail sampling")
		}
	}
	return nil
}

func defaultSamplingConfig() SamplingConfig {
	tail := defaultTailSamplingConfig()
	return SamplingConfig{
		// In a future major release we will set this to
		// false, and then later remove the option.
		KeepUnsampled: true,
		Tail:          &tail,
	}
}

func defaultTailSamplingConfig() TailSamplingConfig {
	return TailSamplingConfig{
		Enabled:               false,
		ESConfig:              elasticsearch.DefaultConfig(),
		DefaultSampleRate:     0.5,
		Interval:              1 * time.Minute,
		IngestRateDecayFactor: 0.25,
		StorageDir:            "tail_sampling",
		StorageGCInterval:     5 * time.Minute,
		TTL:                   30 * time.Minute,
	}
}
