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
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/outputs"
)

const defaultPort = "8200"

type Config struct {
	Host                string                 `config:"host"`
	MaxUnzippedSize     int64                  `config:"max_unzipped_size"`
	MaxHeaderSize       int                    `config:"max_header_size"`
	ReadTimeout         time.Duration          `config:"read_timeout"`
	WriteTimeout        time.Duration          `config:"write_timeout"`
	ShutdownTimeout     time.Duration          `config:"shutdown_timeout"`
	SecretToken         string                 `config:"secret_token"`
	SSL                 *SSLConfig             `config:"ssl"`
	ConcurrentRequests  int                    `config:"concurrent_requests" validate:"min=1"`
	MaxConnections      int                    `config:"max_connections"`
	MaxRequestQueueTime time.Duration          `config:"max_request_queue_time"`
	Expvar              *ExpvarConfig          `config:"expvar"`
	Metrics             *metricsConfig         `config:"metrics"`
	AugmentEnabled      bool                   `config:"capture_personal_data"`
	SelfInstrumentation *InstrumentationConfig `config:"instrumentation"`

	RumConfig      *rumConfig `config:"rum"`
	FrontendConfig *rumConfig `config:"frontend"`
	rum            *rumConfig
	rumOnce        sync.Once

	beatVersion string
}

type ExpvarConfig struct {
	Enabled *bool  `config:"enabled"`
	Url     string `config:"url"`
}

type rumConfig struct {
	Enabled             *bool          `config:"enabled"`
	RateLimit           int            `config:"rate_limit"`
	AllowOrigins        []string       `config:"allow_origins"`
	LibraryPattern      string         `config:"library_pattern"`
	ExcludeFromGrouping string         `config:"exclude_from_grouping"`
	SourceMapping       *SourceMapping `config:"source_mapping"`
}

type metricsConfig struct {
	Enabled *bool `config:"enabled"`
}

type SourceMapping struct {
	Cache        *Cache `config:"cache"`
	IndexPattern string `config:"index_pattern"`

	EsConfig *common.Config `config:"elasticsearch"`
	mapper   sourcemap.Mapper
}

type Cache struct {
	Expiration time.Duration `config:"expiration"`
}

type SSLConfig struct {
	Enabled     *bool                     `config:"enabled"`
	Certificate outputs.CertificateConfig `config:",inline"`
}

type InstrumentationConfig struct {
	Enabled     *bool   `config:"enabled"`
	Environment *string `config:"environment"`
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *ExpvarConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *metricsConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (s *SourceMapping) isSetup() bool {
	return s != nil && (s.EsConfig != nil)
}

func (c *Config) Rum() *rumConfig {
	c.rumOnce.Do(func() {
		if c.RumConfig != nil && c.RumConfig.Enabled != nil {
			c.rum = c.RumConfig
		} else if c.FrontendConfig != nil && c.FrontendConfig.Enabled != nil {
			c.rum = c.FrontendConfig
		} else {
			c.rum = defaultRum()
		}
	})
	return c.rum
}

func (c *Config) isRumEnabled() bool {
	return c != nil && c.Rum().Enabled != nil && *c.Rum().Enabled
}

func (c *Config) setSmapES(esConfig *common.Config) {
	if c != nil && c.Rum().SourceMapping != nil {
		c.Rum().SourceMapping.EsConfig = esConfig
	}
}

func (c *Config) memoizedSmapMapper() (sourcemap.Mapper, error) {
	if c == nil {
		return nil, nil
	}
	if !c.isRumEnabled() || c.Rum().SourceMapping == nil || !c.Rum().SourceMapping.isSetup() {
		return nil, nil
	}

	if c.Rum().SourceMapping.mapper != nil {
		return c.Rum().SourceMapping.mapper, nil
	}

	smap := c.Rum().SourceMapping
	smapConfig := sourcemap.Config{
		CacheExpiration:     smap.Cache.Expiration,
		ElasticsearchConfig: smap.EsConfig,
		Index:               replaceVersion(smap.IndexPattern, c.beatVersion),
	}
	smapMapper, err := sourcemap.NewSmapMapper(smapConfig)
	if err != nil {
		return nil, err
	}

	c.Rum().SourceMapping.mapper = smapMapper
	return smapMapper, nil
}

func (c *InstrumentationConfig) isEnabled() bool {
	// self instrumentation is disabled by default.
	return c != nil && c.Enabled != nil && *c.Enabled
}

func replaceVersion(pattern, version string) string {
	re := regexp.MustCompile("%.*{.*beat.version.?}")
	return re.ReplaceAllLiteralString(pattern, version)
}

func defaultRum() *rumConfig {
	return &rumConfig{
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
	}
}

func defaultConfig(beatVersion string) *Config {
	metricsEnabled := true
	return &Config{
		Host:                net.JoinHostPort("localhost", defaultPort),
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
		beatVersion:         beatVersion,
		Expvar: &ExpvarConfig{
			Enabled: new(bool),
			Url:     "/debug/vars",
		},
		Metrics: &metricsConfig{
			Enabled: &metricsEnabled,
		},
		FrontendConfig: defaultRum(),
		RumConfig:      defaultRum(),
	}
}
