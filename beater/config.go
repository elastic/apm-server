package beater

import (
	"time"

	"github.com/elastic/beats/libbeat/outputs"
)

type Config struct {
	Host               string          `config:"host"`
	MaxUnzippedSize    int64           `config:"max_unzipped_size"`
	MaxHeaderBytes     int             `config:"max_header_bytes"`
	ReadTimeout        time.Duration   `config:"read_timeout"`
	WriteTimeout       time.Duration   `config:"write_timeout"`
	ShutdownTimeout    time.Duration   `config:"shutdown_timeout"`
	SecretToken        string          `config:"secret_token"`
	SSL                *SSLConfig      `config:"ssl"`
	ConcurrentRequests int             `config:"concurrent_requests" validate:"min=1"`
	Frontend           *FrontendConfig `config:"frontend"`
}

type FrontendConfig struct {
	Enabled      *bool    `config:"enabled"`
	RateLimit    int      `config:"rate_limit"`
	AllowOrigins []string `config:"allow_origins"`
}

type SSLConfig struct {
	Enabled     *bool                     `config:"enabled"`
	Certificate outputs.CertificateConfig `config:",inline"`
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *FrontendConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

var defaultConfig = Config{
	Host:               "localhost:8200",
	MaxUnzippedSize:    10 * 1024 * 1024, // 10mb
	MaxHeaderBytes:     1048576,          // 1mb
	ConcurrentRequests: 20,
	ReadTimeout:        2 * time.Second,
	WriteTimeout:       2 * time.Second,
	ShutdownTimeout:    5 * time.Second,
	SecretToken:        "",
	Frontend:           &FrontendConfig{Enabled: new(bool), RateLimit: 10, AllowOrigins: []string{"*"}},
}
