package beater

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
        "max_unzipped_size": 64,
        "max_header_size": 8,
        "read_timeout": 3s,
        "write_timeout": 4s,
        "shutdown_timeout": 9s,
        "secret_token": "1234random",
        "ssl": {
					"enabled": true,
					"key": "1234key",
					"certificate": "1234cert",
				},
        "concurrent_requests": 15,
				"frontend": {
					"enabled": true,
					"rate_limit": 1000,
					"allow_origins": ["example*"],
					"source_mapping": {
						"cache": {
							"expiration": 10m,
						},
						"index_pattern": "apm-test*"
					},
					"library_pattern": "pattern",
					"exclude_from_grouping": "group_pattern",
				}
      }`),
			expectedConfig: Config{
				Host:            "localhost:3000",
				MaxUnzippedSize: 64,
				MaxHeaderSize:   8,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				SecretToken:     "1234random",
				SSL:             &SSLConfig{Enabled: &truthy, PrivateKey: "1234key", Cert: "1234cert"},
				Frontend: &FrontendConfig{
					Enabled:      &truthy,
					RateLimit:    1000,
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 10 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "pattern",
					ExcludeFromGrouping: "group_pattern",
				},
				ConcurrentRequests: 15,
			},
		},
		{
			config: []byte(`{
        "host": "localhost:8200",
        "max_unzipped_size": 64,
        "max_header_size": 8,
        "read_timeout": 3s,
        "write_timeout": 2s,
        "shutdown_timeout": 5s,
				"secret_token": "1234random",
        "concurrent_requests": 20,
				"ssl": {},
				"frontend": {
					"source_mapping": {
					}
				}
      }`),
			expectedConfig: Config{
				Host:               "localhost:8200",
				MaxUnzippedSize:    64,
				MaxHeaderSize:      8,
				ReadTimeout:        3000000000,
				WriteTimeout:       2000000000,
				ShutdownTimeout:    5000000000,
				SecretToken:        "1234random",
				SSL:                &SSLConfig{Enabled: nil, PrivateKey: "", Cert: ""},
				ConcurrentRequests: 20,
				Frontend: &FrontendConfig{
					Enabled:      nil,
					RateLimit:    0,
					AllowOrigins: nil,
					SourceMapping: &SourceMapping{
						IndexPattern: "",
					},
				},
			},
		},
		{
			config: []byte(`{ }`),
			expectedConfig: Config{
				Host:               "",
				MaxUnzippedSize:    0,
				MaxHeaderSize:      0,
				ReadTimeout:        0,
				WriteTimeout:       0,
				ShutdownTimeout:    0,
				SecretToken:        "",
				SSL:                nil,
				ConcurrentRequests: 0,
				Frontend:           nil,
			},
		},
	}
	for idx, test := range cases {
		cfg, err := yaml.NewConfig(test.config)
		assert.NoError(t, err)

		var beaterConfig Config
		err = cfg.Unpack(&beaterConfig)
		assert.NoError(t, err)
		msg := fmt.Sprintf("Test number %v failed. Config: %v, ExpectedConfig: %v", idx, beaterConfig, test.expectedConfig)
		assert.Equal(t, test.expectedConfig, beaterConfig, msg)
	}
}

func TestIsEnabled(t *testing.T) {
	truthy := true
	falsy := false
	cases := []struct {
		config   *SSLConfig
		expected bool
	}{
		{config: nil, expected: false},
		{config: &SSLConfig{Enabled: nil}, expected: true},
		{config: &SSLConfig{Cert: "Cert"}, expected: true},
		{config: &SSLConfig{Cert: "Cert", PrivateKey: "key"}, expected: true},
		{config: &SSLConfig{Cert: "Cert", PrivateKey: "key", Enabled: &falsy}, expected: false},
		{config: &SSLConfig{Enabled: &truthy}, expected: true},
		{config: &SSLConfig{Enabled: &falsy}, expected: false},
	}

	for idx, test := range cases {
		name := fmt.Sprintf("%v %v->%v", idx, test.config, test.expected)
		t.Run(name, func(t *testing.T) {
			b := test.expected
			isEnabled := test.config.isEnabled()
			assert.Equal(t, b, isEnabled, "ssl config but should be %v", b)
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
		{version: "6.2.0", indexPattern: "apm-%{[beat.version]}", replaced: "apm-6.2.0"},
		{version: "6.2.0", indexPattern: "apm-smap", replaced: "apm-smap"},
	}
	for _, test := range cases {
		out := replaceVersion(test.indexPattern, test.version)
		assert.Equal(t, test.replaced, out)
	}
}
