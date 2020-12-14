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

package apmservertest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"
)

const (
	defaultElasticsearchHost = "localhost"
	defaultElasticsearchPort = "9200"
	defaultElasticsearchUser = "apm_server_user"
	defaultElasticsearchPass = "changeme"

	defaultKibanaHost = "localhost"
	defaultKibanaPort = "5601"
	defaultKibanaUser = "apm_user_ro"
	defaultKibanaPass = "changeme"
)

// Config holds APM Server configuration.
type Config struct {
	SecretToken string             `json:"apm-server.secret_token,omitempty"`
	Jaeger      *JaegerConfig      `json:"apm-server.jaeger,omitempty"`
	Kibana      *KibanaConfig      `json:"apm-server.kibana,omitempty"`
	Aggregation *AggregationConfig `json:"apm-server.aggregation,omitempty"`
	Sampling    *SamplingConfig    `json:"apm-server.sampling,omitempty"`
	RUM         *RUMConfig         `json:"apm-server.rum,omitempty"`
	DataStreams *DataStreamsConfig `json:"apm-server.data_streams,omitempty"`
	APIKey      *APIKeyConfig      `json:"apm-server.api_key,omitempty"`

	// ResponseHeaders holds headers to add to all APM Server HTTP responses.
	ResponseHeaders http.Header `json:"apm-server.response_headers,omitempty"`

	// Instrumentation holds configuration for libbeat and apm-server instrumentation.
	Instrumentation *InstrumentationConfig `json:"instrumentation,omitempty"`

	// Logging holds configuration for libbeat logging.
	//
	// We always enable JSON output, and log to stderr.
	Logging *LoggingConfig `json:"logging,omitempty"`

	// Monitoring holds configuration for stack monitoring.
	Monitoring *MonitoringConfig `json:"monitoring,omitempty"`

	// Output holds configuration for the libbeat output.
	Output OutputConfig `json:"output"`

	// Setup holds configuration for libbeat setup.
	Setup *SetupConfig `json:"setup,omitempty"`

	// Queue holds configuration for the libbeat event queue.
	Queue QueueConfig `json:"queue"`
}

// Args formats cfg as a list of arguments to pass to apm-server,
// in the form ["-E", "k=v", "-E", "k=v", ...]
func (cfg Config) Args() ([]string, error) {
	return configArgs(cfg, nil)
}

// LoggingConfig holds APM Server logging configuration.
type LoggingConfig struct {
	Files FileLoggingConfig `json:"files"`
}

// FileLoggingConfig holds APM Server file logging configuration.
type FileLoggingConfig struct {
	Path string `json:"path,omitempty"`
}

// KibanaConfig holds APM Server Kibana connection configuration.
type KibanaConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Host     string `json:"host,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// JaegerConfig holds APM Server Jaeger intake configuration.
type JaegerConfig struct {
	GRPCEnabled bool   `json:"grpc.enabled,omitempty"`
	GRPCHost    string `json:"grpc.host,omitempty"`
	GRPCAuthTag string `json:"grpc.auth_tag,omitempty"`
	HTTPEnabled bool   `json:"http.enabled,omitempty"`
	HTTPHost    string `json:"http.host,omitempty"`
}

// SamplingConfig holds APM Server trace sampling configuration.
type SamplingConfig struct {
	KeepUnsampled bool                `json:"keep_unsampled"`
	Tail          *TailSamplingConfig `json:"tail,omitempty"`
}

// TailSamplingConfig holds APM Server tail-based sampling configuration.
type TailSamplingConfig struct {
	Enabled  bool
	Interval time.Duration
	Policies []TailSamplingPolicy
}

func (t *TailSamplingConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled  bool                 `json:"enabled"`
		Interval string               `json:"interval"`
		Policies []TailSamplingPolicy `json:"policies,omitempty"`
	}
	return json.Marshal(config{
		Enabled:  t.Enabled,
		Interval: durationString(t.Interval),
		Policies: t.Policies,
	})
}

// TailSamplingPolicy holds an APM Server tail-based sampling policy.
type TailSamplingPolicy struct {
	ServiceName        string  `json:"service.name,omitempty"`
	ServiceEnvironment string  `json:"service.environment,omitempty"`
	TraceName          string  `json:"trace.name,omitempty"`
	TraceOutcome       string  `json:"trace.outcome,omitempty"`
	SampleRate         float64 `json:"sample_rate"`
}

// RUMConfig holds APM Server RUM configuration.
type RUMConfig struct {
	Enabled bool `json:"enabled"`

	// ResponseHeaders holds headers to add to all APM Server RUM HTTP responses.
	ResponseHeaders http.Header `json:"response_headers,omitempty"`
}

// DataStreamsConfig holds APM Server data streams configuration.
type DataStreamsConfig struct {
	Enabled bool `json:"enabled"`
}

// APIKeyConfig holds APM Server API Key auth configuration.
type APIKeyConfig struct {
	Enabled bool `json:"enabled"`
}

// InstrumentationConfig holds APM Server instrumentation configuration.
type InstrumentationConfig struct {
	Enabled   bool             `json:"enabled"`
	Profiling *ProfilingConfig `json:"profiling,omitempty"`

	Hosts       []string `json:"hosts,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
	SecretToken string   `json:"secret_token,omitempty"`
}

// ProfilingConfig holds APM Server profiling configuration.
type ProfilingConfig struct {
	CPU  *CPUProfilingConfig  `json:"cpu,omitempty"`
	Heap *HeapProfilingConfig `json:"heap,omitempty"`
}

// CPUProfilingConfig holds APM Server profiling configuration.
type CPUProfilingConfig struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
}

func (c *CPUProfilingConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled  bool   `json:"enabled"`
		Interval string `json:"interval,omitempty"`
		Duration string `json:"duration,omitempty"`
	}
	return json.Marshal(config{
		Enabled:  c.Enabled,
		Interval: durationString(c.Interval),
		Duration: durationString(c.Duration),
	})
}

// HeapProfilingConfig holds APM Server profiling configuration.
type HeapProfilingConfig struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval,omitempty"`
}

func (c *HeapProfilingConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled  bool   `json:"enabled"`
		Interval string `json:"interval,omitempty"`
	}
	return json.Marshal(config{
		Enabled:  c.Enabled,
		Interval: durationString(c.Interval),
	})
}

// OutputConfig holds APM Server libbeat output configuration.
type OutputConfig struct {
	Elasticsearch *ElasticsearchOutputConfig `json:"elasticsearch"`
}

// ElasticsearchOutputConfig holds APM Server libbeat Elasticsearch output configuration.
type ElasticsearchOutputConfig struct {
	Hosts    []string `json:"hosts,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	APIKey   string   `json:"api_key,omitempty"`
}

// SetupConfig holds APM Server libbeat setup configuration.
type SetupConfig struct {
	IndexTemplate IndexTemplateConfig `json:"template.settings.index"`
}

// IndexTemplateConfig holds APM Server libbeat index template setup configuration.
type IndexTemplateConfig struct {
	Shards          int    `json:"number_of_shards,omitempty"`
	Replicas        int    `json:"number_of_replicas"`
	RefreshInterval string `json:"refresh_interval,omitempty"`
}

// QueueConfig holds APM Server libbeat queue configuration.
type QueueConfig struct {
	Memory *MemoryQueueConfig `json:"mem,omitempty"`
}

// MemoryQueueConfig holds APM Server libbeat in-memory queue configuration.
type MemoryQueueConfig struct {
	Events         int
	FlushMinEvents int
	FlushTimeout   time.Duration
}

func (m *MemoryQueueConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Events         int    `json:"events"`
		FlushMinEvents int    `json:"flush.min_events"`
		FlushTimeout   string `json:"flush.timeout,omitempty"`
	}
	return json.Marshal(config{
		Events:         m.Events,
		FlushMinEvents: m.FlushMinEvents,
		FlushTimeout:   durationString(m.FlushTimeout),
	})
}

// MonitoringConfig holds APM Server stack monitoring configuration.
type MonitoringConfig struct {
	Enabled       bool
	Elasticsearch *ElasticsearchOutputConfig
	MetricsPeriod time.Duration
	StatePeriod   time.Duration
}

func (m *MonitoringConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled       bool                       `json:"enabled"`
		Elasticsearch *ElasticsearchOutputConfig `json:"elasticsearch,omitempty"`
		MetricsPeriod string                     `json:"elasticsearch.metrics.period,omitempty"`
		StatePeriod   string                     `json:"elasticsearch.state.period,omitempty"`
	}
	return json.Marshal(config{
		Enabled:       m.Enabled,
		Elasticsearch: m.Elasticsearch,
		MetricsPeriod: durationString(m.MetricsPeriod),
		StatePeriod:   durationString(m.StatePeriod),
	})
}

// AggregationConfig holds APM Server metrics aggregation configuration.
type AggregationConfig struct {
	Transactions        *TransactionAggregationConfig        `json:"transactions,omitempty"`
	ServiceDestinations *ServiceDestinationAggregationConfig `json:"service_destinations,omitempty"`
}

// TransactionAggregationConfig holds APM Server transaction metrics aggregation configuration.
type TransactionAggregationConfig struct {
	Enabled  bool
	Interval time.Duration
}

func (m *TransactionAggregationConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled  bool   `json:"enabled"`
		Interval string `json:"interval,omitempty"`
	}
	return json.Marshal(config{
		Enabled:  m.Enabled,
		Interval: durationString(m.Interval),
	})
}

// ServiceDestinationAggregationConfig holds APM Server service destination metrics aggregation configuration.
type ServiceDestinationAggregationConfig struct {
	Enabled  bool
	Interval time.Duration
}

func (s *ServiceDestinationAggregationConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Enabled  bool   `json:"enabled"`
		Interval string `json:"interval,omitempty"`
	}
	return json.Marshal(config{
		Enabled:  s.Enabled,
		Interval: durationString(s.Interval),
	})
}

func durationString(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

func configArgs(cfg Config, extra map[string]interface{}) ([]string, error) {
	kv, err := flattenConfig(cfg, extra)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(kv)*2)
	for i, kv := range kv {
		out[2*i] = "-E"
		out[2*i+1] = kv
	}
	return out, nil
}

func flattenConfig(cfg Config, extra map[string]interface{}) ([]string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for k, v := range extra {
		m[k] = v
	}

	var kv []string
	flattenMap(m, &kv, "")
	sort.Strings(kv)
	return kv, nil
}

func flattenMap(m map[string]interface{}, out *[]string, prefix string) {
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			flattenMap(v, out, prefix+k+".")
		default:
			jsonv, err := json.Marshal(v)
			if err != nil {
				panic(err)
			}
			*out = append(*out, fmt.Sprintf("%s%s=%s", prefix, k, jsonv))
		}
	}
}

// DefaultConfig holds the default configuration.
func DefaultConfig() Config {
	return Config{
		Kibana: &KibanaConfig{
			Enabled: true,
			Host: (&url.URL{
				Scheme: "http",
				Host: net.JoinHostPort(
					getenvDefault("KIBANA_HOST", defaultKibanaHost),
					DefaultKibanaPort(),
				),
			}).String(),
			Username: getenvDefault("KIBANA_USER", defaultKibanaUser),
			Password: getenvDefault("KIBANA_PASS", defaultKibanaPass),
		},
		Output: OutputConfig{
			Elasticsearch: &ElasticsearchOutputConfig{
				Hosts: []string{net.JoinHostPort(
					getenvDefault("ES_HOST", defaultElasticsearchHost),
					getenvDefault("ES_PORT", defaultElasticsearchPort),
				)},
				Username: getenvDefault("ES_USER", defaultElasticsearchUser),
				Password: getenvDefault("ES_PASS", defaultElasticsearchPass),
			},
		},
		Setup: &SetupConfig{
			IndexTemplate: IndexTemplateConfig{
				Shards:          1,
				RefreshInterval: "250ms",
			},
		},
		Queue: QueueConfig{
			Memory: &MemoryQueueConfig{
				Events:         4096,
				FlushMinEvents: 0, // flush immediately
			},
		},
	}
}

// DefaultKibanaPort returns the Kibana port, configured using
// KIBANA_PORT, or otherwise returning the default of 5601.
func DefaultKibanaPort() string {
	return getenvDefault("KIBANA_PORT", defaultKibanaPort)
}

func getenvDefault(k, defaultv string) string {
	v := os.Getenv(k)
	if v == "" {
		return defaultv
	}
	return v
}
