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
	"go/token"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/transport/tlscommon"

	"github.com/elastic/apm-server/internal/elasticsearch"
)

var (
	noneClientAuth     = tlscommon.TLSClientAuthNone
	optionalClientAuth = tlscommon.TLSClientAuthOptional
	requireClientAuth  = tlscommon.TLSClientAuthRequired
)

var testdataCertificateConfig = tlscommon.CertificateConfig{
	Certificate: "../../testdata/tls/certificate.pem",
	Key:         "../../testdata/tls/key.pem",
}

func TestUnpackConfig(t *testing.T) {
	// When unpacking elastic-agent-libs/kibana.ClientConfig, proxy headers
	// are set to nil rather than an empty map like in the default
	// instantiated value.
	defaultDecodedKibanaClientConfig := defaultKibanaConfig().ClientConfig
	defaultDecodedKibanaClientConfig.Transport.Proxy.Headers = nil

	kibanaNoSlashConfig := DefaultConfig()
	kibanaNoSlashConfig.Kibana.ClientConfig = defaultDecodedKibanaClientConfig
	kibanaNoSlashConfig.Kibana.Enabled = true
	kibanaNoSlashConfig.Kibana.Host = "kibanahost:5601/proxy"

	kibanaHeadersConfig := DefaultConfig()
	kibanaHeadersConfig.Kibana.ClientConfig = defaultDecodedKibanaClientConfig
	kibanaHeadersConfig.Kibana.Enabled = true
	kibanaHeadersConfig.Kibana.Headers = map[string]string{
		"foo":                 "bar",
		"Elastic-Api-Version": "2023-10-31",
	}

	responseHeadersConfig := DefaultConfig()
	responseHeadersConfig.ResponseHeaders = map[string][]string{
		"k1": []string{"v1"},
		"k2": []string{"v2", "v3"},
	}
	responseHeadersConfig.RumConfig.ResponseHeaders = map[string][]string{
		"k4": []string{"v4"},
	}

	reuseOutputESConfig := DefaultConfig()
	reuseOutputESConfig.AgentConfig.ESOverrideConfigured = false
	reuseOutputESConfig.AgentConfig.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9200"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "output_username",
		Password:         "output_password",
		APIKey:           "",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}
	reuseOutputESConfig.RumConfig.Enabled = true
	reuseOutputESConfig.RumConfig.SourceMapping.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9200"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "output_username",
		Password:         "output_password",
		APIKey:           "",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}

	overrideHostOutputESConfig := DefaultConfig()
	overrideHostOutputESConfig.AgentConfig.ESOverrideConfigured = true
	overrideHostOutputESConfig.AgentConfig.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9202"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "output_username",
		Password:         "output_password",
		APIKey:           "",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}
	overrideHostOutputESConfig.RumConfig.Enabled = true
	overrideHostOutputESConfig.RumConfig.SourceMapping.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9201"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "output_username",
		Password:         "output_password",
		APIKey:           "",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}

	overrideCredentialsOutputESConfig := DefaultConfig()
	overrideCredentialsOutputESConfig.AgentConfig.ESOverrideConfigured = true
	overrideCredentialsOutputESConfig.AgentConfig.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9202"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "",
		Password:         "",
		APIKey:           "id2:api_key2",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}
	overrideCredentialsOutputESConfig.RumConfig.Enabled = true
	overrideCredentialsOutputESConfig.RumConfig.SourceMapping.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9201"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Username:         "",
		Password:         "",
		APIKey:           "id:api_key",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}

	overrideCredentialsOutputESConfigMalformed := DefaultConfig()
	overrideCredentialsOutputESConfigMalformed.AgentConfig.ESOverrideConfigured = true
	overrideCredentialsOutputESConfigMalformed.AgentConfig.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9202"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		APIKey:           "id2:api_key2",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}
	overrideCredentialsOutputESConfigMalformed.RumConfig.Enabled = true
	overrideCredentialsOutputESConfigMalformed.RumConfig.SourceMapping.ESConfig = &elasticsearch.Config{
		Hosts:            elasticsearch.Hosts{"localhost:9201"},
		Protocol:         "https",
		Timeout:          5 * time.Second,
		Password:         "foo",
		MaxRetries:       3,
		CompressionLevel: 5,
		Backoff:          elasticsearch.DefaultBackoffConfig,
	}

	tests := map[string]struct {
		inpCfg         map[string]interface{}
		inpOutputESCfg map[string]interface{}
		outCfg         *Config
	}{
		"default config": {
			inpCfg: map[string]interface{}{},
			outCfg: DefaultConfig(),
		},
		"overwrite default": {
			inpCfg: map[string]interface{}{
				"host":                    "localhost:3000",
				"max_header_size":         8,
				"max_event_size":          100,
				"idle_timeout":            5 * time.Second,
				"read_timeout":            3 * time.Second,
				"write_timeout":           4 * time.Second,
				"shutdown_timeout":        9 * time.Second,
				"capture_personal_data":   true,
				"max_concurrent_decoders": 100,
				"auth": map[string]interface{}{
					"secret_token": "1234random",
					"api_key": map[string]interface{}{
						"enabled":             true,
						"limit":               200,
						"elasticsearch.hosts": []string{"localhost:9201", "localhost:9202"},
					},
					"anonymous": map[string]interface{}{
						"enabled":       true,
						"allow_service": []string{"opbeans-rum"},
						"rate_limit": map[string]interface{}{
							"event_limit": 7200,
							"ip_limit":    2000,
						},
					},
				},
				"output": map[string]interface{}{
					"backoff.init": time.Second,
					"backoff.max":  time.Minute,
				},
				"ssl": map[string]interface{}{
					"enabled":                 true,
					"key":                     "../../testdata/tls/key.pem",
					"certificate":             "../../testdata/tls/certificate.pem",
					"certificate_authorities": []string{"../../testdata/tls/ca.crt.pem"},
					"client_authentication":   "required",
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"rum": map[string]interface{}{
					"enabled":       true,
					"allow_origins": []string{"example*"},
					"allow_headers": []string{"Authorization"},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 8 * time.Minute,
						},
						"elasticsearch.hosts": []string{"localhost:9201", "localhost:9202"},
						"timeout":             "2s",
					},
					"library_pattern":       "^custom",
					"exclude_from_grouping": "^grouping",
				},
				"register": map[string]interface{}{
					"ingest": map[string]interface{}{
						"pipeline": map[string]interface{}{
							"overwrite": false,
							"path":      filepath.Join("tmp", "definition.json"),
						},
					},
				},
				"kibana":                        map[string]interface{}{"enabled": "true"},
				"agent.config.cache.expiration": "2m",
				"agent.config.elasticsearch": map[string]interface{}{
					"api_key": "id:api_key",
				},
				"aggregation": map[string]interface{}{
					"max_services": 111,
					"transactions": map[string]interface{}{
						"rollup_intervals": []string{"10s", "10m"},
						"max_groups":       123,
					},
					"service_destinations": map[string]interface{}{
						"max_groups": 456,
					},
					"service_transactions": map[string]interface{}{
						"max_groups": 457,
					},
				},
				"default_service_environment": "overridden",
			},
			outCfg: &Config{
				Host:                  "localhost:3000",
				MaxHeaderSize:         8,
				MaxEventSize:          100,
				IdleTimeout:           5000000000,
				ReadTimeout:           3000000000,
				WriteTimeout:          4000000000,
				ShutdownTimeout:       9000000000,
				MaxConcurrentDecoders: 100,
				AgentAuth: AgentAuth{
					SecretToken: "1234random",
					APIKey: APIKeyAgentAuth{
						Enabled:     true,
						LimitPerMin: 200,
						ESConfig: &elasticsearch.Config{
							Hosts:            elasticsearch.Hosts{"localhost:9201", "localhost:9202"},
							Protocol:         "http",
							Timeout:          5 * time.Second,
							MaxRetries:       3,
							CompressionLevel: 5,
							Backoff:          elasticsearch.DefaultBackoffConfig,
						},
						configured:   true,
						esConfigured: true,
					},
					Anonymous: AnonymousAgentAuth{
						Enabled:      true,
						AllowService: []string{"opbeans-rum"},
						AllowAgent:   []string{"rum-js", "js-base"},
						RateLimit: RateLimit{
							EventLimit: 7200,
							IPLimit:    2000,
						},
						enabledSet: true,
					},
				},
				TLS: &tlscommon.ServerConfig{
					Enabled:     newBool(true),
					Certificate: testdataCertificateConfig,
					ClientAuth:  &requireClientAuth,
					CAs:         []string{"../../testdata/tls/ca.crt.pem"},
				},
				AugmentEnabled: true,
				Expvar: ExpvarConfig{
					Enabled: true,
					URL:     "/debug/vars",
				},
				Pprof: PprofConfig{
					Enabled: false,
				},
				RumConfig: RumConfig{
					Enabled:      true,
					AllowOrigins: []string{"example*"},
					AllowHeaders: []string{"Authorization"},
					SourceMapping: SourceMapping{
						Enabled: true,
						ESConfig: &elasticsearch.Config{
							Hosts:            elasticsearch.Hosts{"localhost:9201", "localhost:9202"},
							Protocol:         "http",
							Timeout:          5 * time.Second,
							MaxRetries:       3,
							CompressionLevel: 5,
							Backoff:          elasticsearch.DefaultBackoffConfig,
						},
						Timeout:              2 * time.Second,
						esOverrideConfigured: true,
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
				},
				Kibana: KibanaConfig{
					Enabled:      true,
					ClientConfig: defaultDecodedKibanaClientConfig,
				},
				AgentConfig: AgentConfig{
					ESConfig: &elasticsearch.Config{
						Hosts:            elasticsearch.Hosts{"localhost:9200"},
						Protocol:         "http",
						Timeout:          5 * time.Second,
						APIKey:           "id:api_key",
						MaxRetries:       3,
						CompressionLevel: 5,
						Backoff:          elasticsearch.DefaultBackoffConfig,
					},
					Cache:                Cache{Expiration: 2 * time.Minute},
					ESOverrideConfigured: true,
				},
				Aggregation: AggregationConfig{
					MaxServices: 111,
					Transactions: TransactionAggregationConfig{
						MaxGroups: 123,
					},
					ServiceDestinations: ServiceDestinationAggregationConfig{
						MaxGroups: 456,
					},
					ServiceTransactions: ServiceTransactionAggregationConfig{
						MaxGroups: 457,
					},
				},
				Sampling: SamplingConfig{
					Tail: TailSamplingConfig{
						Enabled:               false,
						ESConfig:              elasticsearch.DefaultConfig(),
						Interval:              1 * time.Minute,
						IngestRateDecayFactor: 0.25,
						StorageLimit:          "0",
						StorageLimitParsed:    0,
						DiskUsageThreshold:    0.8,
						TTL:                   30 * time.Minute,
					},
				},
				DefaultServiceEnvironment: "overridden",
				DataStreams: DataStreamsConfig{
					Namespace: "default",
				},
				WaitReadyInterval: 5 * time.Second,
			},
		},
		"merge config with default": {
			inpCfg: map[string]interface{}{
				"host": "localhost:3000",
				"auth": map[string]interface{}{
					"api_key.enabled": true,
					"secret_token":    "1234random",
				},
				"ssl": map[string]interface{}{
					"enabled":     true,
					"certificate": "../../testdata/tls/certificate.pem",
					"key":         "../../testdata/tls/key.pem",
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"pprof": map[string]interface{}{
					"enabled": true,
				},
				"rum": map[string]interface{}{
					"enabled": true,
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 7,
						},
					},
					"library_pattern": "rum",
				},
				"register": map[string]interface{}{
					"ingest": map[string]interface{}{
						"pipeline": map[string]interface{}{
							"enabled": false,
						},
					},
				},
				"sampling.tail": map[string]interface{}{
					"enabled":              false,
					"policies":             []map[string]interface{}{{"sample_rate": 0.5}},
					"interval":             "2m",
					"ingest_rate_decay":    1.0,
					"storage_limit":        "1GB",
					"disk_usage_threshold": 0.8,
				},
				"data_streams": map[string]interface{}{
					"namespace": "foo",
				},
			},
			outCfg: &Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   1048576,
				MaxEventSize:    307200,
				IdleTimeout:     45000000000,
				ReadTimeout:     30000000000,
				WriteTimeout:    30000000000,
				ShutdownTimeout: 30000000000,
				AgentAuth: AgentAuth{
					SecretToken: "1234random",
					APIKey: APIKeyAgentAuth{
						Enabled:     true,
						LimitPerMin: 100,
						ESConfig:    elasticsearch.DefaultConfig(),
						configured:  true,
					},
					Anonymous: AnonymousAgentAuth{
						Enabled:    true,
						AllowAgent: []string{"rum-js", "js-base"},
						RateLimit: RateLimit{
							EventLimit: 300,
							IPLimit:    1000,
						},
					},
				},
				TLS: &tlscommon.ServerConfig{
					Enabled:     newBool(true),
					Certificate: testdataCertificateConfig,
				},
				AugmentEnabled: true,
				Expvar: ExpvarConfig{
					Enabled: true,
					URL:     "/debug/vars",
				},
				Pprof: PprofConfig{
					Enabled: true,
				},
				RumConfig: RumConfig{
					Enabled:      true,
					AllowOrigins: []string{"*"},
					AllowHeaders: []string{},
					SourceMapping: SourceMapping{
						Enabled:  true,
						ESConfig: elasticsearch.DefaultConfig(),
						Timeout:  5 * time.Second,
					},
					LibraryPattern:      "rum",
					ExcludeFromGrouping: "^/webpack",
				},
				Kibana: defaultKibanaConfig(),
				AgentConfig: AgentConfig{
					ESConfig: elasticsearch.DefaultConfig(),
					Cache:    Cache{Expiration: 30 * time.Second},
				},
				Aggregation: AggregationConfig{
					MaxServices: 0, // Default value is set as per memory limit
					Transactions: TransactionAggregationConfig{
						MaxGroups: 0, // Default value is set as per memory limit
					},
					ServiceDestinations: ServiceDestinationAggregationConfig{
						MaxGroups: 0, // Default value is set as per memory limit
					},
					ServiceTransactions: ServiceTransactionAggregationConfig{
						MaxGroups: 0, // Default value is set as per memory limit
					},
				},
				Sampling: SamplingConfig{
					Tail: TailSamplingConfig{
						Enabled:               false,
						Policies:              []TailSamplingPolicy{{SampleRate: 0.5}},
						ESConfig:              elasticsearch.DefaultConfig(),
						Interval:              2 * time.Minute,
						IngestRateDecayFactor: 1.0,
						StorageLimit:          "1GB",
						StorageLimitParsed:    1000000000,
						DiskUsageThreshold:    0.8,
						TTL:                   30 * time.Minute,
					},
				},
				DataStreams: DataStreamsConfig{
					Namespace: "foo",
				},
				WaitReadyInterval: 5 * time.Second,
			},
		},
		"kibana trailing slash": {
			inpCfg: map[string]interface{}{
				"kibana": map[string]interface{}{
					"enabled": "true",
					"host":    "kibanahost:5601/proxy/",
				},
			},
			outCfg: kibanaNoSlashConfig,
		},
		"kibana headers": {
			inpCfg: map[string]interface{}{
				"kibana": map[string]interface{}{
					"enabled": "true",
					"headers": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			outCfg: kibanaHeadersConfig,
		},
		"response headers": {
			inpCfg: map[string]interface{}{
				"response_headers": map[string]interface{}{
					"k1": "v1",
					"k2": []string{"v2", "v3"},
				},
				"rum": map[string]interface{}{
					"response_headers": map[string]interface{}{
						"k4": []string{"v4"},
					},
				},
			},
			outCfg: responseHeadersConfig,
		},
		"agentcfg and rum sourcemapping reuse output es config": {
			inpCfg: map[string]interface{}{
				"rum.enabled": true,
			},
			inpOutputESCfg: map[string]interface{}{
				"hosts":    []string{"localhost:9200"},
				"username": "output_username",
				"password": "output_password",
				"protocol": "https",
			},
			outCfg: reuseOutputESConfig,
		},
		"agentcfg and rum sourcemapping override host output es config ": {
			inpCfg: map[string]interface{}{
				"rum.enabled": true,
				"rum.source_mapping.elasticsearch": map[string]interface{}{
					"hosts": []string{"localhost:9201"},
				},
				"agent.config.elasticsearch": map[string]interface{}{
					"hosts": []string{"localhost:9202"},
				},
			},
			inpOutputESCfg: map[string]interface{}{
				"hosts":    []string{"localhost:9200"},
				"username": "output_username",
				"password": "output_password",
				"protocol": "https",
			},
			outCfg: overrideHostOutputESConfig,
		},
		"agentcfg and rum sourcemapping override credentials output es config ": {
			inpCfg: map[string]interface{}{
				"rum.enabled": true,
				"rum.source_mapping.elasticsearch": map[string]interface{}{
					"hosts":   []string{"localhost:9201"},
					"api_key": "id:api_key",
					"invalid": "invalid",
				},
				"agent.config.elasticsearch": map[string]interface{}{
					"hosts":   []string{"localhost:9202"},
					"api_key": "id2:api_key2",
					"invalid": "invalid",
				},
			},
			inpOutputESCfg: map[string]interface{}{
				"hosts":    []string{"localhost:9200"},
				"username": "output_username",
				"password": "output_password",
				"protocol": "https",
			},
			outCfg: overrideCredentialsOutputESConfig,
		},
		"agentcfg and rum sourcemapping override credentials output es config malformed": {
			inpCfg: map[string]interface{}{
				"rum.enabled": true,
				"rum.source_mapping.elasticsearch": map[string]interface{}{
					"hosts":    []string{"localhost:9201"},
					"password": "foo",
					"invalid":  "invalid",
				},
				"agent.config.elasticsearch": map[string]interface{}{
					"hosts":   []string{"localhost:9202"},
					"api_key": "id2:api_key2",
					"invalid": "invalid",
				},
			},
			inpOutputESCfg: map[string]interface{}{
				"hosts":    []string{"localhost:9200"},
				"username": "output_username",
				"password": "output_password",
				"protocol": "https",
			},
			outCfg: overrideCredentialsOutputESConfigMalformed,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			inpCfg, err := config.NewConfigFrom(test.inpCfg)
			assert.NoError(t, err)

			var inpOutputESCfg *config.C
			if test.inpOutputESCfg != nil {
				inpOutputESCfg, err = config.NewConfigFrom(test.inpOutputESCfg)
				assert.NoError(t, err)
			}

			cfg, err := NewConfig(inpCfg, inpOutputESCfg)
			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Config should match exactly, with the exception of unexported fields.
			if diff := cmp.Diff(test.outCfg, cfg, ignoreUnexported()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestTLSSettings(t *testing.T) {
	t.Run("ClientAuthentication", func(t *testing.T) {
		for name, tc := range map[string]struct {
			config map[string]interface{}

			tls *tlscommon.ServerConfig
		}{
			"Defaults": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"enabled":     true,
					"key":         "../../testdata/tls/key.pem",
					"certificate": "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{Certificate: testdataCertificateConfig},
			},
			"ConfiguredToRequired": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication": "required",
					"key":                   "../../testdata/tls/key.pem",
					"certificate":           "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: &requireClientAuth, Certificate: testdataCertificateConfig},
			},
			"ConfiguredToOptional": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication": "optional",
					"key":                   "../../testdata/tls/key.pem",
					"certificate":           "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: &optionalClientAuth, Certificate: testdataCertificateConfig},
			},
			"DefaultRequiredByCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"certificate_authorities": []string{"../../testdata/tls/ca.crt.pem"},
					"key":                     "../../testdata/tls/key.pem",
					"certificate":             "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: &requireClientAuth, Certificate: testdataCertificateConfig},
			},
			"ConfiguredWithCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication":   "none",
					"certificate_authorities": []string{"../../testdata/tls/ca.crt.pem"},
					"key":                     "../../testdata/tls/key.pem",
					"certificate":             "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: &noneClientAuth, Certificate: testdataCertificateConfig},
			},
		} {
			t.Run(name, func(t *testing.T) {
				ucfgCfg, err := config.NewConfigFrom(tc.config)
				require.NoError(t, err)

				cfg, err := NewConfig(ucfgCfg, nil)
				require.NoError(t, err)
				assert.Equal(t, tc.tls.ClientAuth, cfg.TLS.ClientAuth)
			})
		}
	})

	t.Run("Enabled", func(t *testing.T) {
		for name, tc := range map[string]struct {
			tlsServerCfg *tlscommon.ServerConfig
			expected     bool
		}{
			"NoConfig":          {tlsServerCfg: nil, expected: false},
			"SSL":               {tlsServerCfg: &tlscommon.ServerConfig{Enabled: nil}, expected: true},
			"WithCert":          {tlsServerCfg: &tlscommon.ServerConfig{Certificate: tlscommon.CertificateConfig{Certificate: "Cert"}}, expected: true},
			"WithCertAndKey":    {tlsServerCfg: &tlscommon.ServerConfig{Certificate: tlscommon.CertificateConfig{Certificate: "Cert", Key: "key"}}, expected: true},
			"ConfiguredToFalse": {tlsServerCfg: &tlscommon.ServerConfig{Certificate: tlscommon.CertificateConfig{Certificate: "Cert", Key: "key"}, Enabled: newBool(false)}, expected: false},
			"ConfiguredToTrue":  {tlsServerCfg: &tlscommon.ServerConfig{Enabled: newBool(true)}, expected: true},
		} {
			t.Run(name, func(t *testing.T) {
				b := tc.expected
				isEnabled := tc.tlsServerCfg.IsEnabled()
				assert.Equal(t, b, isEnabled)
			})
		}
	})
}

func TestNewConfig_ESConfig(t *testing.T) {
	ucfg, err := config.NewConfigFrom(`{
		"rum.enabled": true,
		"auth.api_key.enabled": true,
		"sampling.tail.policies": [{"sample_rate": 0.5}],
	}`)
	require.NoError(t, err)

	// no es config given
	cfg, err := NewConfig(ucfg, nil)
	require.NoError(t, err)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.AgentAuth.APIKey.ESConfig)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.Sampling.Tail.ESConfig)

	// with es config
	outputESCfg := config.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`)
	cfg, err = NewConfig(ucfg, outputESCfg)
	require.NoError(t, err)
	assert.NotNil(t, cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.RumConfig.SourceMapping.ESConfig.Hosts))
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.AgentAuth.APIKey.ESConfig.Hosts))
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.Sampling.Tail.ESConfig.Hosts))
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.AgentConfig.ESConfig.Hosts))

}

func newBool(v bool) *bool {
	return &v
}

func ignoreUnexported() cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool {
		field, ok := p.Index(-1).(cmp.StructField)
		if !ok {
			return false
		}
		return !token.IsExported(field.Name())
	}, cmp.Ignore())
}
