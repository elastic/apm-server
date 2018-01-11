package beater

import (
	"time"

	"github.com/elastic/apm-server/sourcemap"
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
	Cache *Cache `config:"cache"`
	Index string `config:"index"`

	esConfig *common.Config
	mapper   sourcemap.Mapper
}

type Cache struct {
	Expiration time.Duration `config:"expiration"`
}

type SSLConfig struct {
	Enabled    *bool  `config:"enabled"`
	PrivateKey string `config:"key"`
	Cert       string `config:"certificate"`
}

func (c *Config) setElasticsearch(esConfig *common.Config) {
	if c != nil && c.Frontend.isEnabled() && c.Frontend.Sourcemapping != nil {
		c.Frontend.Sourcemapping.esConfig = esConfig
	}
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *FrontendConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (s *Sourcemapping) isSetup() bool {
	return s != nil && (s.esConfig != nil)
}

func (c *FrontendConfig) SmapMapper() (sourcemap.Mapper, error) {
	smap := c.Sourcemapping
	if !c.isEnabled() || !smap.isSetup() {
		return nil, nil
	}
	if smap.mapper != nil {
		return c.Sourcemapping.mapper, nil
	}
	smapConfig := sourcemap.Config{
		CacheExpiration:     smap.Cache.Expiration,
		ElasticsearchConfig: smap.esConfig,
		Index:               smap.Index + "*",
	}
	smapMapper, err := sourcemap.NewSmapMapper(smapConfig)
	if err != nil {
		return nil, err
	}
	c.Sourcemapping.mapper = smapMapper
	return c.Sourcemapping.mapper, nil
}

func defaultConfig() *Config {
	return &Config{
		Host:               "localhost:8200",
		MaxUnzippedSize:    50 * 1024 * 1024, // 50mb
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
					Expiration: 5 * time.Minute,
				},
				Index: "apm",
			},
		},
	}
}
