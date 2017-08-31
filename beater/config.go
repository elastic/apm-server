package beater

import (
	"time"
)

type Config struct {
	Host               string        `config:"host"`
	MaxUnzippedSize    int64         `config:"max_unzipped_size"`
	MaxHeaderBytes     int           `config:"max_header_bytes"`
	ReadTimeout        time.Duration `config:"read_timeout"`
	WriteTimeout       time.Duration `config:"write_timeout"`
	ShutdownTimeout    time.Duration `config:"shutdown_timeout"`
	SecretToken        string        `config:"secret_token"`
	SSL                *SSLConfig    `config:"ssl"`
	ConcurrentRequests int           `config:"concurrent_requests" validate:"min=1"`
}

type SSLConfig struct {
	Enabled    *bool  `config:"enabled"`
	PrivateKey string `config:"key"`
	Cert       string `config:"certificate"`
}

func (c *SSLConfig) isEnabled() bool {
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
}
