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
	"fmt"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/elastic/beats/libbeat/common/transport/tlscommon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/outputs"

	"github.com/elastic/apm-server/elasticsearch"
)

func Test_UnpackConfig(t *testing.T) {
	falsy, truthy := false, true
	version := "8.0.0"

	tests := map[string]struct {
		inpCfg map[string]interface{}
		outCfg *Config
	}{
		"default config": {
			inpCfg: map[string]interface{}{},
			outCfg: DefaultConfig(version),
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
				"ssl": map[string]interface{}{
					"enabled":                 true,
					"key":                     path.Join("../..", "testdata", "tls", "key.pem"),
					"certificate":             path.Join("../..", "testdata", "tls", "certificate.pem"),
					"certificate_authorities": []string{path.Join("../..", "testdata", "tls", "./ca.crt.pem")},
					"client_authentication":   "none",
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
					"allow_origins": []string{"example*"},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 8 * time.Minute,
						},
						"index_pattern": "apm-test*",
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
			},
			outCfg: &Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   8,
				MaxEventSize:    100,
				IdleTimeout:     5000000000,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				SecretToken:     "1234random",
				TLS: &tlscommon.ServerConfig{
					Enabled: &truthy,
					Certificate: outputs.CertificateConfig{
						Certificate: path.Join("../..", "testdata", "tls", "certificate.pem"),
						Key:         path.Join("../..", "testdata", "tls", "key.pem")},
					ClientAuth: 0,
					CAs:        []string{path.Join("../..", "testdata", "tls", "./ca.crt.pem")},
				},
				AugmentEnabled: true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					URL:     "/debug/vars",
				},
				RumConfig: &RumConfig{
					Enabled: &truthy,
					EventRate: &EventRate{
						Limit:   7200,
						LruSize: 2000,
					},
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 8 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
					BeatVersion:         version,
				},
				Register: &RegisterConfig{
					Ingest: &IngestConfig{
						Pipeline: &PipelineConfig{
							Enabled:   &truthy,
							Overwrite: &falsy,
							Path:      filepath.Join("tmp", "definition.json"),
						},
					},
				},
				Kibana:      common.MustNewConfigFrom(map[string]interface{}{"enabled": "true"}),
				AgentConfig: &AgentConfig{Cache: &Cache{Expiration: 2 * time.Minute}},
				Pipeline:    defaultAPMPipeline,
				JaegerConfig: JaegerConfig{
					GRPC: JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:12345",
						TLS: func() *tls.Config {
							tlsServerConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled: &truthy,
								Certificate: outputs.CertificateConfig{
									Certificate: path.Join("../..", "testdata", "tls", "certificate.pem"),
									Key:         path.Join("../..", "testdata", "tls", "key.pem")},
								ClientAuth: 0,
								CAs:        []string{path.Join("../..", "testdata", "tls", "./ca.crt.pem")}})
							require.NoError(t, err)
							return tlsServerConfig.BuildModuleConfig("localhost:12345")
						}(),
					},
					HTTP: JaegerHTTPConfig{
						Enabled: true,
						Host:    "localhost:6789",
					},
				},
				APIKeyConfig: &APIKeyConfig{
					Enabled:     true,
					LimitPerMin: 200,
					ESConfig:    &elasticsearch.Config{Hosts: elasticsearch.Hosts{"localhost:9201", "localhost:9202"}},
				},
			},
		},
		"merge config with default": {
			inpCfg: map[string]interface{}{
				"host":         "localhost:3000",
				"secret_token": "1234random",
				"ssl": map[string]interface{}{
					"enabled": true,
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
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
				"jaeger.grpc.enabled": true,
				"api_key.enabled":     true,
			},
			outCfg: &Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   1048576,
				MaxEventSize:    307200,
				IdleTimeout:     45000000000,
				ReadTimeout:     30000000000,
				WriteTimeout:    30000000000,
				ShutdownTimeout: 5000000000,
				SecretToken:     "1234random",
				TLS: &tlscommon.ServerConfig{
					Enabled:     &truthy,
					Certificate: outputs.CertificateConfig{Certificate: "", Key: ""},
					ClientAuth:  3},
				AugmentEnabled: true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					URL:     "/debug/vars",
				},
				RumConfig: &RumConfig{
					Enabled: &truthy,
					EventRate: &EventRate{
						Limit:   300,
						LruSize: 1000,
					},
					AllowOrigins: []string{"*"},
					SourceMapping: &SourceMapping{
						Cache: &Cache{
							Expiration: 7 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
					},
					LibraryPattern:      "rum",
					ExcludeFromGrouping: "^/webpack",
					BeatVersion:         "8.0.0",
				},
				Register: &RegisterConfig{
					Ingest: &IngestConfig{
						Pipeline: &PipelineConfig{
							Enabled: &falsy,
							Path:    filepath.Join("ingest", "pipeline", "definition.json"),
						},
					},
				},
				Kibana:      common.MustNewConfigFrom(map[string]interface{}{"enabled": "false"}),
				AgentConfig: &AgentConfig{Cache: &Cache{Expiration: 30 * time.Second}},
				Pipeline:    defaultAPMPipeline,
				JaegerConfig: JaegerConfig{
					GRPC: JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:14250",
						TLS: func() *tls.Config {
							tlsServerConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled:     &truthy,
								Certificate: outputs.CertificateConfig{Certificate: "", Key: ""},
								ClientAuth:  3})
							require.NoError(t, err)
							return tlsServerConfig.BuildModuleConfig("localhost:14250")
						}(),
					},
					HTTP: JaegerHTTPConfig{
						Enabled: false,
						Host:    "localhost:14268",
					},
				},
				APIKeyConfig: &APIKeyConfig{Enabled: true, LimitPerMin: 100, ESConfig: elasticsearch.DefaultConfig()},
			},
		},
	}

	for name, test := range tests {
		t.Run(name+"no outputESCfg", func(t *testing.T) {
			inpCfg, err := common.NewConfigFrom(test.inpCfg)
			assert.NoError(t, err)

			cfg, err := NewConfig(version, inpCfg, nil)
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, test.outCfg, cfg)
		})
	}
}

func TestReplaceBeatVersion(t *testing.T) {
	cases := []struct {
		version      string
		indexPattern string
		replaced     string
	}{
		{version: "", indexPattern: "", replaced: ""},
		{version: "6.2.0", indexPattern: "apm-%{[observer.version]}", replaced: "apm-6.2.0"},
		{version: "6.2.0", indexPattern: "apm-sourcemap", replaced: "apm-sourcemap"},
	}
	for _, test := range cases {
		out := replaceVersion(test.indexPattern, test.version)
		assert.Equal(t, test.replaced, out)
	}
}

func TestPipeline(t *testing.T) {
	truthy, falsy := true, false
	cases := []struct {
		c                  *PipelineConfig
		enabled, overwrite bool
	}{
		{c: nil, enabled: false, overwrite: false},
		{c: &PipelineConfig{}, enabled: true, overwrite: false}, //default values
		{c: &PipelineConfig{Enabled: &falsy, Overwrite: &truthy},
			enabled: false, overwrite: true},
		{c: &PipelineConfig{Enabled: &truthy, Overwrite: &falsy},
			enabled: true, overwrite: false},
	}

	for idx, test := range cases {
		assert.Equal(t, test.enabled, test.c.IsEnabled(),
			fmt.Sprintf("<%v> IsEnabled() expected %v", idx, test.enabled))
		assert.Equal(t, test.overwrite, test.c.ShouldOverwrite(),
			fmt.Sprintf("<%v> ShouldOverwrite() expected %v", idx, test.overwrite))
	}
}

func TestTLSSettings(t *testing.T) {

	t.Run("ClientAuthentication", func(t *testing.T) {
		for name, tc := range map[string]struct {
			config map[string]interface{}

			tls *tlscommon.ServerConfig
		}{
			"Defaults": {
				config: map[string]interface{}{"ssl": nil},
				tls:    &tlscommon.ServerConfig{ClientAuth: 3},
			},
			"ConfiguredToRequired": {
				config: map[string]interface{}{"ssl": map[string]interface{}{"client_authentication": "required"}},
				tls:    &tlscommon.ServerConfig{ClientAuth: 4},
			},
			"ConfiguredToNone": {
				config: map[string]interface{}{"ssl": map[string]interface{}{"client_authentication": "none"}},
				tls:    &tlscommon.ServerConfig{ClientAuth: 0},
			},
			"DefaultRequiredByCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"certificate_authorities": []string{path.Join("..", "..", "testdata", "tls", "./ca.crt.pem")}}},
				tls: &tlscommon.ServerConfig{ClientAuth: 4},
			},
			"ConfiguredWithCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"certificate_authorities": []string{path.Join("..", "..", "testdata", "tls", "./ca.crt.pem")},
					"client_authentication":   "none"}},
				tls: &tlscommon.ServerConfig{ClientAuth: 0},
			},
		} {
			t.Run(name, func(t *testing.T) {
				ucfgCfg, err := common.NewConfigFrom(tc.config)
				require.NoError(t, err)

				cfg, err := NewConfig("9.9.9", ucfgCfg, nil)
				require.NoError(t, err)
				assert.Equal(t, tc.tls.ClientAuth, cfg.TLS.ClientAuth)
			})
		}
	})

	t.Run("VerificationMode", func(t *testing.T) {
		for name, tc := range map[string]struct {
			config map[string]interface{}
			tls    *tlscommon.ServerConfig
		}{
			"Default": {
				config: map[string]interface{}{"ssl": nil},
				tls:    &tlscommon.ServerConfig{VerificationMode: tlscommon.VerifyFull}},
			"ConfiguredToFull": {
				config: map[string]interface{}{"ssl": map[string]interface{}{"verification_mode": "full"}},
				tls:    &tlscommon.ServerConfig{VerificationMode: tlscommon.VerifyFull}},
			"ConfiguredToNone": {
				config: map[string]interface{}{"ssl": map[string]interface{}{"verification_mode": "none"}},
				tls:    &tlscommon.ServerConfig{VerificationMode: tlscommon.VerifyNone}},
		} {
			t.Run(name, func(t *testing.T) {
				ucfgCfg, err := common.NewConfigFrom(tc.config)
				require.NoError(t, err)

				cfg, err := NewConfig("9.9.9", ucfgCfg, nil)
				require.NoError(t, err)
				assert.Equal(t, tc.tls.VerificationMode, cfg.TLS.VerificationMode)
			})
		}
	})

	t.Run("Enabled", func(t *testing.T) {
		truthy := true
		falsy := false
		for name, tc := range map[string]struct {
			tlsServerCfg *tlscommon.ServerConfig
			expected     bool
		}{
			"NoConfig":          {tlsServerCfg: nil, expected: false},
			"SSL":               {tlsServerCfg: &tlscommon.ServerConfig{Enabled: nil}, expected: true},
			"WithCert":          {tlsServerCfg: &tlscommon.ServerConfig{Certificate: outputs.CertificateConfig{Certificate: "Cert"}}, expected: true},
			"WithCertAndKey":    {tlsServerCfg: &tlscommon.ServerConfig{Certificate: outputs.CertificateConfig{Certificate: "Cert", Key: "key"}}, expected: true},
			"ConfiguredToFalse": {tlsServerCfg: &tlscommon.ServerConfig{Certificate: outputs.CertificateConfig{Certificate: "Cert", Key: "key"}, Enabled: &falsy}, expected: false},
			"ConfiguredToTrue":  {tlsServerCfg: &tlscommon.ServerConfig{Enabled: &truthy}, expected: true},
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
		cfg, err := NewConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("InvalidUnit", func(t *testing.T) {
		cfg, err := NewConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "1230ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg, err := NewConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123000ms"}), nil)
		require.NoError(t, err)
		assert.Equal(t, time.Second*123, cfg.AgentConfig.Cache.Expiration)
	})
}

func TestNewConfig_ESConfig(t *testing.T) {
	version := "8.0.0"
	ucfg, err := common.NewConfigFrom(`{"rum.enabled":true,"api_key.enabled":true}`)
	require.NoError(t, err)

	// no es config given
	cfg, err := NewConfig(version, ucfg, nil)
	require.NoError(t, err)
	assert.Nil(t, cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, elasticsearch.DefaultConfig(), cfg.APIKeyConfig.ESConfig)

	// with es config
	outputESCfg := common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`)
	cfg, err = NewConfig(version, ucfg, outputESCfg)
	require.NoError(t, err)
	assert.NotNil(t, cfg.RumConfig.SourceMapping.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.RumConfig.SourceMapping.ESConfig.Hosts))
	assert.NotNil(t, cfg.APIKeyConfig.ESConfig)
	assert.Equal(t, []string{"192.0.0.168:9200"}, []string(cfg.APIKeyConfig.ESConfig.Hosts))
}
