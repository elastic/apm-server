package config

import (
	"time"

	"github.com/elastic/beats/libbeat/logp"
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
	Hosts       urls            `config:"hosts" validate:"nonzero"`
	Profiling   ProfilingConfig `config:"profiling"`
	SecretToken string          `config:"secret_token"`
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
