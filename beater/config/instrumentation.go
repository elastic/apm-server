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

	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/go-ucfg"
)

const (
	defaultCPUProfilingInterval  = 1 * time.Minute
	defaultCPUProfilingDuration  = 10 * time.Second
	defaultHeapProfilingInterval = 1 * time.Minute
)

// InstrumentationConfig holds config information about self instrumenting the APM Server
type InstrumentationConfig struct {
	Enabled     *bool           `config:"enabled"`
	Environment *string         `config:"environment"`
	Hosts       urls            `config:"hosts"` //TODO(simi): add `validate:"nonzero"` again once https://github.com/elastic/go-ucfg/issues/147 is fixed
	Profiling   ProfilingConfig `config:"profiling"`
	APIKey      string          `config:"api_key"`
	SecretToken string          `config:"secret_token"`
}

func (c *InstrumentationConfig) Validate() error {
	for _, h := range c.Hosts {
		if h == nil || h.Host == "" {
			return ucfg.ErrZeroValue
		}
	}
	return nil
}

// IsEnabled indicates whether self instrumentation is enabled
func (c *InstrumentationConfig) IsEnabled() bool {
	// self instrumentation is disabled by default.
	return c != nil && c.Enabled != nil && *c.Enabled
}

func (c *InstrumentationConfig) setup(log *logp.Logger) error {
	if !c.IsEnabled() {
		return nil
	}
	if err := c.Profiling.CPU.setup(log); err != nil {
		return err
	}
	if err := c.Profiling.Heap.setup(log); err != nil {
		return err
	}
	return nil
}

// ProfilingConfig holds config information about self profiling the APM Server
type ProfilingConfig struct {
	CPU  *CPUProfiling  `config:"cpu"`
	Heap *HeapProfiling `config:"heap"`
}

// CPUProfiling holds config information about CPU profiling of the APM Server
type CPUProfiling struct {
	Enabled  bool          `config:"enabled"`
	Interval time.Duration `config:"interval" validate:"positive"`
	Duration time.Duration `config:"duration" validate:"positive"`
}

// IsEnabled indicates whether CPU profiling is enabled or not
func (p *CPUProfiling) IsEnabled() bool {
	return p != nil && p.Enabled
}

func (p *CPUProfiling) setup(log *logp.Logger) error {
	if !p.IsEnabled() {
		return nil
	}
	if p.Interval <= 0 {
		p.Interval = defaultCPUProfilingInterval
	}
	if p.Duration <= 0 {
		p.Duration = defaultCPUProfilingDuration
	}
	return nil
}

// HeapProfiling holds config information about heap profiling of the APM Server
type HeapProfiling struct {
	Enabled  bool          `config:"enabled"`
	Interval time.Duration `config:"interval" validate:"positive"`
}

// IsEnabled indicates whether heap profiling is enabled or not
func (p *HeapProfiling) IsEnabled() bool {
	return p != nil && p.Enabled
}

func (p *HeapProfiling) setup(log *logp.Logger) error {
	if !p.IsEnabled() {
		return nil
	}
	if p.Interval <= 0 {
		p.Interval = defaultHeapProfilingInterval
	}
	return nil
}
