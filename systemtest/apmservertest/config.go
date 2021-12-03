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
	Kibana                    *KibanaConfig      `json:"apm-server.kibana,omitempty"`
	Aggregation               *AggregationConfig `json:"apm-server.aggregation,omitempty"`
	Sampling                  *SamplingConfig    `json:"apm-server.sampling,omitempty"`
	RUM                       *RUMConfig         `json:"apm-server.rum,omitempty"`
	WaitForIntegration        *bool              `json:"apm-server.data_streams.wait_for_integration,omitempty"`
	DefaultServiceEnvironment string             `json:"apm-server.default_service_environment,omitempty"`
	KibanaAgentConfig         *KibanaAgentConfig `json:"apm-server.agent.config,omitempty"`
	TLS                       *TLSConfig         `json:"apm-server.ssl,omitempty"`

	// AgentAuth holds configuration for APM agent authorization.
	AgentAuth AgentAuthConfig `json:"apm-server.auth"`

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
}

// Args formats cfg as a list of arguments to pass to apm-server,
// in the form ["-E", "k=v", "-E", "k=v", ...]
func (cfg Config) Args() ([]string, error) {
	return configArgs(cfg, nil)
}

// TLSConfig holds configuration to TLS encryption of agent/server communication.
type TLSConfig struct {
	// ClientAuthentication controls whether TLS client authentication is
	// enabled, and optional or required. If this is non-empty, then
	// `apm-server.ssl.certificate_authorities` will be set to the server's
	// self-signed certificate path.
	ClientAuthentication string `json:"client_authentication,omitempty"`

	CipherSuites       []string `json:"cipher_suites,omitempty"`
	SupportedProtocols []string `json:"supported_protocols,omitempty"`
}

// KibanaAgentConfig holds configuration related to the Kibana-based
// implementation of agent configuration.
type KibanaAgentConfig struct {
	CacheExpiration time.Duration
}

func (c *KibanaAgentConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		CacheExpiration string `json:"cache.expiration,omitempty"`
	}
	return json.Marshal(config{
		CacheExpiration: durationString(c.CacheExpiration),
	})
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

// SamplingConfig holds APM Server trace sampling configuration.
type SamplingConfig struct {
	Tail *TailSamplingConfig `json:"tail,omitempty"`
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

	// AllowOrigins holds a list of allowed origins for RUM.
	AllowOrigins []string `json:"allow_origins,omitempty"`

	// AllowHeaders holds a list of Access-Control-Allow-Headers for RUM.
	AllowHeaders []string `json:"allow_headers,omitempty"`

	// AllowServiceNames holds a list of exclusively allowed service names for
	// RUM events.
	AllowServiceNames []string `json:"allow_service_names,omitempty"`

	// ResponseHeaders holds headers to add to all APM Server RUM HTTP responses.
	ResponseHeaders http.Header `json:"response_headers,omitempty"`

	Sourcemap *RUMSourcemapConfig `json:"source_mapping,omitempty"`
}

// RUMSourcemapConfig holds APM Server RUM sourcemap configuration.
type RUMSourcemapConfig struct {
	Enabled bool                     `json:"enabled,omitempty"`
	Cache   *RUMSourcemapCacheConfig `json:"cache,omitempty"`
}

// RUMSourcemapCacheConfig holds sourcemap cache expiration.
type RUMSourcemapCacheConfig struct {
	Expiration time.Duration `json:"expiration,omitempty"`
}

// APIKeyConfig holds agent auth configuration.
type AgentAuthConfig struct {
	SecretToken string               `json:"secret_token,omitempty"`
	APIKey      *APIKeyAuthConfig    `json:"api_key,omitempty"`
	Anonymous   *AnonymousAuthConfig `json:"anonymous,omitempty"`
}

// APIKeyAuthConfig holds API Key agent auth configuration.
type APIKeyAuthConfig struct {
	Enabled bool `json:"enabled"`
}

// AnonymousAuthConfig holds anonymous agent auth configuration.
type AnonymousAuthConfig struct {
	Enabled      bool             `json:"enabled"`
	AllowAgent   []string         `json:"allow_agent,omitempty"`
	AllowService []string         `json:"allow_service,omitempty"`
	RateLimit    *RateLimitConfig `json:"rate_limit,omitempty"`
}

// RateLimitConfig holds event rate limit configuration.
type RateLimitConfig struct {
	IPLimit    int `json:"ip_limit,omitempty"`
	EventLimit int `json:"event_limit,omitempty"`
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
	Console       *ConsoleOutputConfig       `json:"console,omitempty"`
	Elasticsearch *ElasticsearchOutputConfig `json:"elasticsearch,omitempty"`
}

// ConsoleOutputConfig holds APM Server libbeat console output configuration.
type ConsoleOutputConfig struct {
	Enabled bool `json:"enabled"`
}

// ElasticsearchOutputConfig holds APM Server libbeat Elasticsearch output configuration.
type ElasticsearchOutputConfig struct {
	Enabled  bool     `json:"enabled"`
	Hosts    []string `json:"hosts,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	APIKey   string   `json:"api_key,omitempty"`

	// modelindexer settings
	FlushBytes    string        `json:"flush_bytes,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty"`
}

func (c *ElasticsearchOutputConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Enabled       bool     `json:"enabled"`
		Hosts         []string `json:"hosts,omitempty"`
		Username      string   `json:"username,omitempty"`
		Password      string   `json:"password,omitempty"`
		APIKey        string   `json:"api_key,omitempty"`
		FlushBytes    string   `json:"flush_bytes,omitempty"`
		FlushInterval string   `json:"flush_interval,omitempty"`
	}{
		Enabled:       c.Enabled,
		Hosts:         c.Hosts,
		Username:      c.Username,
		Password:      c.Password,
		APIKey:        c.APIKey,
		FlushBytes:    c.FlushBytes,
		FlushInterval: durationString(c.FlushInterval),
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
	Interval time.Duration
}

func (m *TransactionAggregationConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Interval string `json:"interval,omitempty"`
	}
	return json.Marshal(config{
		Interval: durationString(m.Interval),
	})
}

// ServiceDestinationAggregationConfig holds APM Server service destination metrics aggregation configuration.
type ServiceDestinationAggregationConfig struct {
	Interval time.Duration
}

func (s *ServiceDestinationAggregationConfig) MarshalJSON() ([]byte, error) {
	// time.Duration is encoded as int64.
	// Convert time.Durations to durations, to encode as duration strings.
	type config struct {
		Interval string `json:"interval,omitempty"`
	}
	return json.Marshal(config{
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
					KibanaPort(),
				),
			}).String(),
			Username: getenvDefault("KIBANA_USER", defaultKibanaUser),
			Password: getenvDefault("KIBANA_PASS", defaultKibanaPass),
		},
		Output: defaultOutputConfig(),
	}
}

// defaultOutputConfig enables overriding the default output, and is used to
// default to console output in tests for apmservertest itself. This is needed
// to avoid interacting with Elasticsearch, which would cause systemtest tests
// to fail as they assume sole access to Elasticsearch.
func defaultOutputConfig() OutputConfig {
	var outputConfig OutputConfig
	switch v := os.Getenv("APMSERVERTEST_DEFAULT_OUTPUT"); v {
	case "console":
		outputConfig.Console = &ConsoleOutputConfig{Enabled: true}
	case "":
		outputConfig.Elasticsearch = &ElasticsearchOutputConfig{
			Enabled: true,
			Hosts: []string{net.JoinHostPort(
				getenvDefault("ES_HOST", defaultElasticsearchHost),
				ElasticsearchPort(),
			)},
			Username: getenvDefault("ES_USER", defaultElasticsearchUser),
			Password: getenvDefault("ES_PASS", defaultElasticsearchPass),
			// Lower the flush interval to 1ms to avoid delaying the tests.
			FlushInterval: time.Millisecond,
		}
	default:
		panic("APMSERVERTEST_DEFAULT_OUTPUT has unexpected value: " + v)
	}
	return outputConfig
}

// KibanaPort returns the Kibana port, configured using
// KIBANA_PORT, or otherwise returning the default of 5601.
func KibanaPort() string {
	return getenvDefault("KIBANA_PORT", defaultKibanaPort)
}

// ElasticsearchPort returns the Elasticsearch REST API port,
// configured using ES_PORT, or otherwise returning the default
// of 9200.
func ElasticsearchPort() string {
	return getenvDefault("ES_PORT", defaultElasticsearchPort)
}

func getenvDefault(k, defaultv string) string {
	v := os.Getenv(k)
	if v == "" {
		return defaultv
	}
	return v
}
