package beater

import (
	"regexp"
	"time"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

type Config struct {
	Host                string          `config:"host"`
	MaxUnzippedSize     int64           `config:"max_unzipped_size"`
	MaxHeaderSize       int             `config:"max_header_size"`
	ReadTimeout         time.Duration   `config:"read_timeout"`
	WriteTimeout        time.Duration   `config:"write_timeout"`
	ShutdownTimeout     time.Duration   `config:"shutdown_timeout"`
	SecretToken         string          `config:"secret_token"`
	SSL                 *SSLConfig      `config:"ssl"`
	ConcurrentRequests  int             `config:"concurrent_requests" validate:"min=1"`
	MaxConnections      int             `config:"max_connections"`
	MaxRequestQueueTime time.Duration   `config:"max_request_queue_time"`
	Expvar              *ExpvarConfig   `config:"expvar"`
	Frontend            *FrontendConfig `config:"frontend"`
	AugmentEnabled      bool            `config:"capture_personal_data"`
}

type ExpvarConfig struct {
	Enabled *bool  `config:"enabled"`
	Url     string `config:"url"`
}

type FrontendConfig struct {
	Enabled             *bool          `config:"enabled"`
	RateLimit           int            `config:"rate_limit"`
	AllowOrigins        []string       `config:"allow_origins"`
	LibraryPattern      string         `config:"library_pattern"`
	ExcludeFromGrouping string         `config:"exclude_from_grouping"`
	SourceMapping       *SourceMapping `config:"source_mapping"`

	beatVersion string
}

type SourceMapping struct {
	Cache        *Cache `config:"cache"`
	IndexPattern string `config:"index_pattern"`

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
	if c != nil && c.Frontend.isEnabled() && c.Frontend.SourceMapping != nil {
		c.Frontend.SourceMapping.esConfig = esConfig
	}
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *ExpvarConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *FrontendConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (s *SourceMapping) isSetup() bool {
	return s != nil && (s.esConfig != nil)
}

func (c *FrontendConfig) memoizedSmapMapper() (sourcemap.Mapper, error) {
	smap := c.SourceMapping
	if !c.isEnabled() || !smap.isSetup() {
		return nil, nil
	}
	if smap.mapper != nil {
		return c.SourceMapping.mapper, nil
	}
	smapConfig := sourcemap.Config{
		CacheExpiration:     smap.Cache.Expiration,
		ElasticsearchConfig: smap.esConfig,
		Index:               replaceVersion(c.SourceMapping.IndexPattern, c.beatVersion),
	}
	smapMapper, err := sourcemap.NewSmapMapper(smapConfig)
	if err != nil {
		return nil, err
	}
	c.SourceMapping.mapper = smapMapper
	return c.SourceMapping.mapper, nil
}

func replaceVersion(pattern, version string) string {
	re := regexp.MustCompile("%.*{.*beat.version.?}")
	return re.ReplaceAllLiteralString(pattern, version)
}

func defaultConfig(beatVersion string) *Config {
	return &Config{
		Host:                "localhost:8200",
		MaxUnzippedSize:     30 * 1024 * 1024, // 30mb
		MaxHeaderSize:       1 * 1024 * 1024,  // 1mb
		ConcurrentRequests:  5,
		MaxConnections:      0, // unlimited
		MaxRequestQueueTime: 2 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		SecretToken:         "",
		AugmentEnabled:      true,
		Frontend: &FrontendConfig{
			beatVersion:  beatVersion,
			Enabled:      new(bool),
			RateLimit:    10,
			AllowOrigins: []string{"*"},
			SourceMapping: &SourceMapping{
				Cache: &Cache{
					Expiration: 5 * time.Minute,
				},
				IndexPattern: "apm-*-sourcemap*",
			},
			LibraryPattern:      "node_modules|bower_components|~",
			ExcludeFromGrouping: "^/webpack",
		},
		Expvar: &ExpvarConfig{
			Enabled: new(bool),
			Url:     "/debug/vars",
		},
	}
}
