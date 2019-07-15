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

package beater

import (
	"fmt"
	"testing"
	"time"

	"github.com/elastic/beats/libbeat/common/transport/tlscommon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/outputs"
	"github.com/elastic/go-ucfg/yaml"
)

func TestConfig(t *testing.T) {
	truthy := true
	cases := []struct {
		config         []byte
		expectedConfig Config
	}{
		{
			config: []byte(`{
        "host": "localhost:3000",
        "max_header_size": 8,
        "idle_timeout": 4s,
        "read_timeout": 3s,
        "write_timeout": 4s,
        "shutdown_timeout": 9s,
        "secret_token": "1234random",
        "ssl": {
			"enabled": true,
			"key": "1234key",
			"certificate": "1234cert",
			"certificate_authorities": ["./ca.cert.pem"]
		},
		"rum": {
			"enabled": true,
			"event_rate": {
				"limit":      8000,
				"lru_size": 2000,
			},
			"allow_origins": ["rum*"],
			"source_mapping": {
				"cache": {
					"expiration": 1m,
				},
				"index_pattern": "apm-rum-test*"
			},
			"library_pattern": "pattern-rum",
			"exclude_from_grouping": "group_pattern-rum",
		},
		"register": {
			"ingest": { 
				"pipeline": {
					enabled: true,
					overwrite: true,
					path: "tmp",
				}
			}
		}
      }`),
			expectedConfig: Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   8,
				IdleTimeout:     4000000000,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				SecretToken:     "1234random",
				TLS: &tlscommon.ServerConfig{
					Enabled:     &truthy,
					CAs:         []string{"./ca.cert.pem"},
					Certificate: outputs.CertificateConfig{Certificate: "1234cert", Key: "1234key"},
					ClientAuth:  4}, //4=RequireAndVerifyClientCert
				RumConfig: &rumConfig{
					Enabled: &truthy,
					EventRate: &eventRate{
						Limit:   8000,
						LruSize: 2000,
					},
					AllowOrigins: []string{"rum*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 1 * time.Minute},
						IndexPattern: "apm-rum-test*",
					},
					LibraryPattern:      "pattern-rum",
					ExcludeFromGrouping: "group_pattern-rum",
				},
				Register: &registerConfig{
					Ingest: &ingestConfig{
						Pipeline: &pipelineConfig{
							Enabled:   &truthy,
							Overwrite: &truthy,
							Path:      "tmp",
						},
					},
				},
			},
		},
		{
			config: []byte(`{
        "host": "localhost:8200",
        "max_header_size": 8,
        "read_timeout": 3s,
        "write_timeout": 2s,
        "shutdown_timeout": 5s,
        "secret_token": "1234random",
		"rum": {
			"source_mapping": {}
		},
		"register": {}
      }`),
			expectedConfig: Config{
				Host:            "localhost:8200",
				MaxHeaderSize:   8,
				ReadTimeout:     3000000000,
				WriteTimeout:    2000000000,
				ShutdownTimeout: 5000000000,
				SecretToken:     "1234random",
				RumConfig: &rumConfig{
					Enabled:      nil,
					EventRate:    nil,
					AllowOrigins: nil,
					SourceMapping: &SourceMapping{
						IndexPattern: "",
					},
				},
				Register: &registerConfig{
					Ingest: nil,
				},
			},
		},
		{
			config: []byte(`{ }`),
			expectedConfig: Config{
				Host:            "",
				MaxHeaderSize:   0,
				IdleTimeout:     0,
				ReadTimeout:     0,
				WriteTimeout:    0,
				ShutdownTimeout: 0,
				SecretToken:     "",
				TLS:             nil,
				RumConfig:       nil,
			},
		},
	}
	for idx, test := range cases {
		cfg, err := yaml.NewConfig(test.config)
		require.NoError(t, err)
		var beaterConfig Config
		err = cfg.Unpack(&beaterConfig)
		require.NoError(t, err)

		msg := fmt.Sprintf("Test number %v failed. Config: %v, ExpectedConfig: %v", idx, beaterConfig, test.expectedConfig)
		assert.Equal(t, test.expectedConfig, beaterConfig, msg)
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
		{version: "6.2.0", indexPattern: "apm-smap", replaced: "apm-smap"},
	}
	for _, test := range cases {
		out := replaceVersion(test.indexPattern, test.version)
		assert.Equal(t, test.replaced, out)
	}
}

func TestIsRumEnabled(t *testing.T) {
	truthy := true
	for _, td := range []struct {
		c       *Config
		enabled bool
	}{
		{c: &Config{RumConfig: &rumConfig{Enabled: new(bool)}}, enabled: false},
		{c: &Config{RumConfig: &rumConfig{Enabled: &truthy}}, enabled: true},
	} {
		assert.Equal(t, td.enabled, td.c.RumConfig.isEnabled())

	}
}

func TestDefaultRum(t *testing.T) {
	c := defaultConfig("7.0.0")
	assert.Equal(t, c.RumConfig, defaultRum("7.0.0"))
}

func TestMemoizedSmapMapper(t *testing.T) {
	truthy := true
	esConfig, err := common.NewConfigFrom(map[string]interface{}{
		"hosts": []string{"localhost:0"},
	})
	require.NoError(t, err)
	smapping := SourceMapping{
		Cache:        &Cache{Expiration: 1 * time.Minute},
		IndexPattern: "apm-rum-test*",
		EsConfig:     esConfig,
	}

	for idx, td := range []struct {
		c       *Config
		smapper bool
		e       error
	}{
		{c: &Config{RumConfig: &rumConfig{}}, smapper: false, e: nil},
		{c: &Config{RumConfig: &rumConfig{Enabled: new(bool)}}, smapper: false, e: nil},
		{c: &Config{RumConfig: &rumConfig{Enabled: &truthy}}, smapper: false, e: nil},
		{c: &Config{RumConfig: &rumConfig{SourceMapping: &smapping}}, smapper: false, e: nil},
		{c: &Config{
			RumConfig: &rumConfig{
				Enabled: &truthy,
				SourceMapping: &SourceMapping{
					Cache:        &Cache{Expiration: 1 * time.Minute},
					IndexPattern: "apm-rum-test*",
				},
			}},
			smapper: false,
			e:       nil},
		{c: &Config{RumConfig: &rumConfig{Enabled: &truthy, SourceMapping: &smapping}},
			smapper: true,
			e:       nil},
	} {
		smapper, e := td.c.RumConfig.memoizedSmapMapper()
		if td.smapper {
			assert.NotNil(t, smapper, fmt.Sprintf("Test number <%v> failed", idx))
		} else {
			assert.Nil(t, smapper, fmt.Sprintf("Test number <%v> failed", idx))
		}
		assert.Equal(t, td.e, e)
	}
}

func TestPipeline(t *testing.T) {
	truthy, falsy := true, false
	cases := []struct {
		c                  *pipelineConfig
		enabled, overwrite bool
	}{
		{c: nil, enabled: false, overwrite: false},
		{c: &pipelineConfig{}, enabled: true, overwrite: false}, //default values
		{c: &pipelineConfig{Enabled: &falsy, Overwrite: &truthy},
			enabled: false, overwrite: true},
		{c: &pipelineConfig{Enabled: &truthy, Overwrite: &falsy},
			enabled: true, overwrite: false},
	}

	for idx, test := range cases {
		assert.Equal(t, test.enabled, test.c.isEnabled(),
			fmt.Sprintf("<%v> isEnabled() expected %v", idx, test.enabled))
		assert.Equal(t, test.overwrite, test.c.shouldOverwrite(),
			fmt.Sprintf("<%v> shouldOverwrite() expected %v", idx, test.overwrite))
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
					"certificate_authorities": []string{"./path"}}},
				tls: &tlscommon.ServerConfig{ClientAuth: 4},
			},
			"ConfiguredWithCA": {
				config: map[string]interface{}{"ssl": map[string]interface{}{
					"certificate_authorities": []string{"./path"}, "client_authentication": "none"}},
				tls: &tlscommon.ServerConfig{ClientAuth: 0},
			},
		} {
			t.Run(name, func(t *testing.T) {
				ucfgCfg, err := common.NewConfigFrom(tc.config)
				require.NoError(t, err)

				cfg, err := newConfig("9.9.9", ucfgCfg)
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

				cfg, err := newConfig("9.9.9", ucfgCfg)
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
		cfg, err := newConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123ms"}))
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("InvalidUnit", func(t *testing.T) {
		cfg, err := newConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "1230ms"}))
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg, err := newConfig("9.9.9",
			common.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123000ms"}))
		require.NoError(t, err)
		assert.Equal(t, time.Second*123, cfg.AgentConfig.Cache.Expiration)
	})
}
