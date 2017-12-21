package beater

import (
	"time"

	"github.com/elastic/beats/libbeat/common"
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
	Enabled       *bool          `config:"enabled"`
	RateLimit     int            `config:"rate_limit"`
	AllowOrigins  []string       `config:"allow_origins"`
	Sourcemapping *Sourcemapping `config:"sourcemapping"`
}

type Sourcemapping struct {
	Cache         *Cache `config:"cache"`
	Index         string `config:"index"`
	Elasticsearch *common.Config
}

type Cache struct {
	Expiration      time.Duration `config:"expiration"`
	CleanupInterval time.Duration `config:"cleanup_interval"`
}

type SSLConfig struct {
	Enabled    *bool  `config:"enabled"`
	PrivateKey string `config:"key"`
	Cert       string `config:"certificate"`
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *FrontendConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (s *Sourcemapping) isSetup() bool {
	return s != nil && (s.Elasticsearch != nil)
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
	Frontend: &FrontendConfig{
		Enabled:      new(bool),
		RateLimit:    10,
		AllowOrigins: []string{"*"},
		Sourcemapping: &Sourcemapping{
			Cache: &Cache{
				Expiration:      300,
				CleanupInterval: 600,
			},
			Index: "apm",
		},
	},
}
