package beater

import (
	"fmt"
	"testing"

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
        "max_header_bytes": 8,
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
      }`),
			expectedConfig: Config{
				Host:               "localhost:3000",
				MaxUnzippedSize:    64,
				MaxHeaderBytes:     8,
				ReadTimeout:        3000000000,
				WriteTimeout:       4000000000,
				ShutdownTimeout:    9000000000,
				SecretToken:        "1234random",
				SSL:                &SSLConfig{Enabled: &truthy, PrivateKey: "1234key", Cert: "1234cert"},
				ConcurrentRequests: 15,
			},
		},
		{
			config: []byte(`{
        "host": "localhost:8200",
        "max_unzipped_size": 64,
        "max_header_bytes": 8,
        "read_timeout": 3s,
        "write_timeout": 2s,
        "shutdown_timeout": 5s,
				"secret_token": "1234random",
        "concurrent_requests": 20,
      }`),
			expectedConfig: Config{
				Host:               "localhost:8200",
				MaxUnzippedSize:    64,
				MaxHeaderBytes:     8,
				ReadTimeout:        3000000000,
				WriteTimeout:       2000000000,
				ShutdownTimeout:    5000000000,
				SecretToken:        "1234random",
				SSL:                nil,
				ConcurrentRequests: 20,
			},
		},
		{
			config: []byte(`{ }`),
			expectedConfig: Config{
				Host:               "",
				MaxUnzippedSize:    0,
				MaxHeaderBytes:     0,
				ReadTimeout:        0,
				WriteTimeout:       0,
				ShutdownTimeout:    0,
				SecretToken:        "",
				SSL:                nil,
				ConcurrentRequests: 0,
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
