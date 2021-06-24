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
	"crypto/tls"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"

	"github.com/elastic/apm-server/elasticsearch"
)

var testdataCertificateConfig = tlscommon.CertificateConfig{
	Certificate: "../../testdata/tls/certificate.pem",
	Key:         "../../testdata/tls/key.pem",
}

func TestUnpackConfig(t *testing.T) {
	kibanaNoSlashConfig := DefaultConfig()
	kibanaNoSlashConfig.Kibana.Enabled = true
	kibanaNoSlashConfig.Kibana.Host = "kibanahost:5601/proxy"

	kibanaHeadersConfig := DefaultConfig()
	kibanaHeadersConfig.Kibana.Enabled = true
	kibanaHeadersConfig.Kibana.Headers = map[string]string{"foo": "bar"}

	responseHeadersConfig := DefaultConfig()
	responseHeadersConfig.ResponseHeaders = map[string][]string{
		"k1": []string{"v1"},
		"k2": []string{"v2", "v3"},
	}
	responseHeadersConfig.RumConfig.ResponseHeaders = map[string][]string{
		"k4": []string{"v4"},
	}

	tests := map[string]struct {
		inpCfg map[string]interface{}
		outCfg *Config
	}{
		"default config": {
			inpCfg: map[string]interface{}{},
			outCfg: DefaultConfig(),
		},
		"overwrite default": {
			inpCfg: map[string]interface{}{
				"host":                  "localhost:3000",
				"max_header_size":       8,
				"max_event_size":        100,
				"idle_timeout":          5 * time.Second,
				"read_timeout":          3 * time.Second,
				"write_timeout":         4 * time.Second,
				"shutdown_timeout":      9 * time.Second,
				"capture_personal_data": true,
				"secret_token":          "1234random",
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
					"enabled": true,
					"event_rate": map[string]interface{}{
						"limit":    7200,
						"lru_size": 2000,
					},
					"allow_service_names": []string{"opbeans-rum"},
					"allow_origins":       []string{"example*"},
					"allow_headers":       []string{"Authorization"},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 8 * time.Minute,
						},
						"index_pattern":       "apm-test*",
						"elasticsearch.hosts": []string{"localhost:9201", "localhost:9202"},
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
				"jaeger.grpc.enabled":           true,
				"jaeger.grpc.host":              "localhost:12345",
				"jaeger.http.enabled":           true,
				"jaeger.http.host":              "localhost:6789",
				"api_key": map[string]interface{}{
					"enabled":             true,
					"limit":               200,
					"elasticsearch.hosts": []string{"localhost:9201", "localhost:9202"},
				},
				"aggregation": map[string]interface{}{
					"transactions": map[string]interface{}{
						"enabled":                          false,
						"interval":                         "1s",
						"max_groups":                       123,
						"hdrhistogram_significant_figures": 1,
					},
					"service_destinations": map[string]interface{}{
						"max_groups": 456,
					},
				},
				"default_service_environment": "overridden",
			},
			outCfg: &Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   8,
				MaxEventSize:    100,
				IdleTimeout:     5000000000,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				AgentAuth: AgentAuth{
					SecretToken: "1234random",
					APIKey: APIKeyAgentAuth{
						Enabled:     true,
						LimitPerMin: 200,
						ESConfig: &elasticsearch.Config{
							Hosts:      elasticsearch.Hosts{"localhost:9201", "localhost:9202"},
							Protocol:   "http",
							Timeout:    5 * time.Second,
							MaxRetries: 3,
							Backoff:    elasticsearch.DefaultBackoffConfig,
						},
						configured:   true,
						esConfigured: true,
					},
				},
				TLS: &tlscommon.ServerConfig{
					Enabled:     newBool(true),
					Certificate: testdataCertificateConfig,
					ClientAuth:  4,
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
				SelfInstrumentation: InstrumentationConfig{
					Profiling: ProfilingConfig{
						CPU: CPUProfiling{
							Interval: 1 * time.Minute,
							Duration: 10 * time.Second,
						},
						Heap: HeapProfiling{
							Interval: 1 * time.Minute,
						},
					},
				},
				RumConfig: RumConfig{
					Enabled: true,
					EventRate: EventRate{
						Limit:   7200,
						LruSize: 2000,
					},
					AllowServiceNames: []string{"opbeans-rum"},
					AllowOrigins:      []string{"example*"},
					AllowHeaders:      []string{"Authorization"},
					SourceMapping: SourceMapping{
						Enabled:      true,
						Cache:        Cache{Expiration: 8 * time.Minute},
						IndexPattern: "apm-test*",
						ESConfig: &elasticsearch.Config{
							Hosts:      elasticsearch.Hosts{"localhost:9201", "localhost:9202"},
							Protocol:   "http",
							Timeout:    5 * time.Second,
							MaxRetries: 3,
							Backoff:    elasticsearch.DefaultBackoffConfig,
						},
						Metadata:     []SourceMapMetadata{},
						esConfigured: true,
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
				},
				Register: RegisterConfig{
					Ingest: IngestConfig{
						Pipeline: PipelineConfig{
							Enabled:   true,
							Overwrite: false,
							Path:      filepath.Join("tmp", "definition.json"),
						},
					},
				},
				Kibana: KibanaConfig{
					Enabled:      true,
					ClientConfig: defaultKibanaConfig().ClientConfig,
				},
				KibanaAgentConfig: KibanaAgentConfig{Cache: Cache{Expiration: 2 * time.Minute}},
				Pipeline:          defaultAPMPipeline,
				JaegerConfig: JaegerConfig{
					GRPC: JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:12345",
						TLS: func() *tls.Config {
							tlsServerConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled:     newBool(true),
								Certificate: testdataCertificateConfig,
								ClientAuth:  4,
								CAs:         []string{"../../testdata/tls/ca.crt.pem"}})
							require.NoError(t, err)
							return tlsServerConfig.BuildServerConfig("localhost:12345")
						}(),
					},
					HTTP: JaegerHTTPConfig{
						Enabled: true,
						Host:    "localhost:6789",
					},
				},
				Aggregation: AggregationConfig{
					Transactions: TransactionAggregationConfig{
						Enabled:                        false,
						Interval:                       time.Second,
						MaxTransactionGroups:           123,
						HDRHistogramSignificantFigures: 1,
					},
					ServiceDestinations: ServiceDestinationAggregationConfig{
						Enabled:   true,
						Interval:  time.Minute,
						MaxGroups: 456,
					},
				},
				Sampling: SamplingConfig{
					KeepUnsampled: true,
					Tail: TailSamplingConfig{
						Enabled:               false,
						ESConfig:              elasticsearch.DefaultConfig(),
						Interval:              1 * time.Minute,
						IngestRateDecayFactor: 0.25,
						StorageDir:            "tail_sampling",
						StorageGCInterval:     5 * time.Minute,
						TTL:                   30 * time.Minute,
					},
				},
				DefaultServiceEnvironment: "overridden",
			},
		},
		"merge config with default": {
			inpCfg: map[string]interface{}{
				"host":         "localhost:3000",
				"secret_token": "1234random",
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
						"metadata": []map[string]string{
							{
								"service.name":    "opbeans-rum",
								"service.version": "1.2.3",
								"bundle.filepath": "/test/e2e/general-usecase/bundle.js.map",
								"sourcemap.url":   "http://somewhere.com/bundle.js.map",
							},
						},
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
				"jaeger.grpc.enabled":                      true,
				"api_key.enabled":                          true,
				"aggregation.transactions.enabled":         false,
				"aggregation.service_destinations.enabled": false,
				"sampling.keep_unsampled":                  false,
				"sampling.tail": map[string]interface{}{
					"enabled":           true,
					"policies":          []map[string]interface{}{{"sample_rate": 0.5}},
					"interval":          "2m",
					"ingest_rate_decay": 1.0,
				},
			},
			outCfg: &Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   1048576,
				MaxEventSize:    307200,
				IdleTimeout:     45000000000,
				ReadTimeout:     30000000000,
				WriteTimeout:    30000000000,
				ShutdownTimeout: 5000000000,
				AgentAuth: AgentAuth{
					SecretToken: "1234random",
					APIKey: APIKeyAgentAuth{
						Enabled:     true,
						LimitPerMin: 100,
						ESConfig:    elasticsearch.DefaultConfig(),
						configured:  true,
					},
				},
				TLS: &tlscommon.ServerConfig{
					Enabled:     newBool(true),
					Certificate: testdataCertificateConfig,
					ClientAuth:  0,
				},
				AugmentEnabled: true,
				Expvar: ExpvarConfig{
					Enabled: true,
					URL:     "/debug/vars",
				},
				Pprof: PprofConfig{
					Enabled: true,
				},
				SelfInstrumentation: InstrumentationConfig{
					Profiling: ProfilingConfig{
						CPU: CPUProfiling{
							Interval: 1 * time.Minute,
							Duration: 10 * time.Second,
						},
						Heap: HeapProfiling{
							Interval: 1 * time.Minute,
						},
					},
				},
				RumConfig: RumConfig{
					Enabled: true,
					EventRate: EventRate{
						Limit:   300,
						LruSize: 1000,
					},
					AllowOrigins: []string{"*"},
					AllowHeaders: []string{},
					SourceMapping: SourceMapping{
						Enabled: true,
						Cache: Cache{
							Expiration: 7 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
						ESConfig:     elasticsearch.DefaultConfig(),
						Metadata: []SourceMapMetadata{
							{
								ServiceName:    "opbeans-rum",
								ServiceVersion: "1.2.3",
								BundleFilepath: "/test/e2e/general-usecase/bundle.js.map",
								SourceMapURL:   "http://somewhere.com/bundle.js.map",
							},
						},
					},
					LibraryPattern:      "rum",
					ExcludeFromGrouping: "^/webpack",
				},
				Register: RegisterConfig{
					Ingest: IngestConfig{
						Pipeline: PipelineConfig{
							Enabled: false,
							Path:    filepath.Join("ingest", "pipeline", "definition.json"),
						},
					},
				},
				Kibana:            defaultKibanaConfig(),
				KibanaAgentConfig: KibanaAgentConfig{Cache: Cache{Expiration: 30 * time.Second}},
				Pipeline:          defaultAPMPipeline,
				JaegerConfig: JaegerConfig{
					GRPC: JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:14250",
						TLS: func() *tls.Config {
							tlsServerConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled:     newBool(true),
								Certificate: testdataCertificateConfig,
								ClientAuth:  0,
							})
							require.NoError(t, err)
							return tlsServerConfig.BuildServerConfig("localhost:14250")
						}(),
					},
					HTTP: JaegerHTTPConfig{
						Enabled: false,
						Host:    "localhost:14268",
					},
				},
				Aggregation: AggregationConfig{
					Transactions: TransactionAggregationConfig{
						Enabled:                        false,
						Interval:                       time.Minute,
						MaxTransactionGroups:           10000,
						HDRHistogramSignificantFigures: 2,
					},
					ServiceDestinations: ServiceDestinationAggregationConfig{
						Enabled:   false,
						Interval:  time.Minute,
						MaxGroups: 10000,
					},
				},
				Sampling: SamplingConfig{
					KeepUnsampled: false,
					Tail: TailSamplingConfig{
						Enabled:               true,
						Policies:              []TailSamplingPolicy{{SampleRate: 0.5}},
						ESConfig:              elasticsearch.DefaultConfig(),
						Interval:              2 * time.Minute,
						IngestRateDecayFactor: 1.0,
						StorageDir:            "tail_sampling",
						StorageGCInterval:     5 * time.Minute,
						TTL:                   30 * time.Minute,
					},
				},
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
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			inpCfg, err := common.NewConfigFrom(test.inpCfg)
			assert.NoError(t, err)

			cfg, err := NewConfig(inpCfg, nil)
			require.NoError(t, err)
			require.NotNil(t, cfg)
			if test.outCfg.JaegerConfig.GRPC.TLS != nil || cfg.JaegerConfig.GRPC.TLS != nil {
				// tlscommon captures closures for the following callbacks
				// setting them to nil to skip these from comparison
				cfg.JaegerConfig.GRPC.TLS.VerifyConnection = nil
				test.outCfg.JaegerConfig.GRPC.TLS.VerifyConnection = nil
				test.outCfg.JaegerConfig.GRPC.TLS.ClientCAs = nil
				cfg.JaegerConfig.GRPC.TLS.ClientCAs = nil
			}

			assert.Equal(t, test.outCfg, cfg)
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
				tls: &tlscommon.ServerConfig{ClientAuth: 0, Certificate: testdataCertificateConfig},
			},
			"ConfiguredToRequired": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication": "required",
					"key":                   "../../testdata/tls/key.pem",
					"certificate":           "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: 4, Certificate: testdataCertificateConfig},
			},
			"ConfiguredToOptional": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication": "optional",
					"key":                   "../../testdata/tls/key.pem",
					"certificate":           "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: 3, Certificate: testdataCertificateConfig},
			},
			"DefaultRequiredByCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"certificate_authorities": []string{"../../testdata/tls/ca.crt.pem"},
					"key":                     "../../testdata/tls/key.pem",
					"certificate":             "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: 4, Certificate: testdataCertificateConfig},
			},
			"ConfiguredWithCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"client_authentication":   "none",
					"certificate_authorities": []string{"../../testdata/tls/ca.crt.pem"},
					"key":                     "../../testdata/tls/key.pem",
					"certificate":             "../../testdata/tls/certificate.pem",
				}},
				tls: &tlscommon.ServerConfig{ClientAuth: 0, Certificate: testdataCertificateConfig},
			},
		} {
			t.Run(name, func(t *testing.T) {
				ucfgCfg, err := common.NewConfigFrom(tc.config)
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

func TestAgentConfig(t *testing.T) {
	t.Run("InvalidValueTooSmall", func(t *testing.T) {
		cfg, err := NewConfig(common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("InvalidUnit", func(t *testing.T) {
		cfg, err := NewConfig(common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "1230ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg, err := NewConfig(common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123000ms"}), nil)
		require.NoError(t, err)
		assert.Equal(t, time.Second*123, cfg.KibanaAgentConfig.Cache.Expiration)
	})
}

func TestNewConfig_ESConfig(t *testing.T) {
	ucfg, err := common.NewConfigFrom(`{"rum.enabled":true,"api_key.enabled":true,"sampling.tail.policies":[{"sample_rate": 0.5}]}`)
	require.NoError(t, err)

	// no es config given
	cfg, err := NewConfig(ucfg, nil)
	require.NoError(t, err)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.AgentAuth.APIKey.ESConfig)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.Sampling.Tail.ESConfig)

	// with es config
	outputESCfg := common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`)
	cfg, err = NewConfig(ucfg, outputESCfg)
	require.NoError(t, err)
	assert.NotNil(t, cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.RumConfig.SourceMapping.ESConfig.Hosts))
	assert.NotNil(t, cfg.AgentAuth.APIKey.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.AgentAuth.APIKey.ESConfig.Hosts))
	assert.NotNil(t, cfg.Sampling.Tail.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.Sampling.Tail.ESConfig.Hosts))
}

func newBool(v bool) *bool {
	return &v
}
