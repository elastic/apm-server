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
	"path/filepath"
	"regexp"
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
	MaxEventSize        int                    `config:"max_event_size"`
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
	RumConfig           *rumConfig             `config:"rum"`
	FrontendConfig      *rumConfig             `config:"frontend"`
	Register            *registerConfig        `config:"register"`
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

	beatVersion string
}

type metricsConfig struct {
	Enabled *bool `config:"enabled"`
}

type registerConfig struct {
	Ingest *ingestConfig `config:"ingest"`
}

type ingestConfig struct {
	Pipeline *pipelineConfig `config:"pipeline"`
}

type pipelineConfig struct {
	Enabled   *bool `config:"enabled"`
	Overwrite *bool `config:"overwrite"`
	Path      string
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

func (c *Config) setSmapElasticsearch(esConfig *common.Config) {
	if c != nil && c.RumConfig.isEnabled() && c.RumConfig.SourceMapping != nil {
		c.RumConfig.SourceMapping.EsConfig = esConfig
	}
}

func (c *SSLConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *ExpvarConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *rumConfig) isEnabled() bool {
	return c != nil && (c.Enabled != nil && *c.Enabled)
}

func (c *metricsConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (s *SourceMapping) isSetup() bool {
	return s != nil && (s.EsConfig != nil)
}

func (c *pipelineConfig) isEnabled() bool {
	return c != nil && (c.Enabled != nil && *c.Enabled)
}

func (c *pipelineConfig) shouldOverwrite() bool {
	return c != nil && (c.Overwrite != nil && *c.Overwrite)
}

func (c *Config) SetRumConfig() {
	if c.RumConfig != nil && c.RumConfig.Enabled != nil {
		return
	}
	c.RumConfig = c.FrontendConfig
}

func (c *rumConfig) memoizedSmapMapper() (sourcemap.Mapper, error) {
	if !c.isEnabled() || !c.SourceMapping.isSetup() {
		return nil, nil
	}
	if c.SourceMapping.mapper != nil {
		return c.SourceMapping.mapper, nil
	}

	smapConfig := sourcemap.Config{
		CacheExpiration:     c.SourceMapping.Cache.Expiration,
		ElasticsearchConfig: c.SourceMapping.EsConfig,
		Index:               replaceVersion(c.SourceMapping.IndexPattern, c.beatVersion),
	}
	smapMapper, err := sourcemap.NewSmapMapper(smapConfig)
	if err != nil {
		return nil, err
	}
	c.SourceMapping.mapper = smapMapper
	return c.SourceMapping.mapper, nil
}

func (c *InstrumentationConfig) isEnabled() bool {
	// self instrumentation is disabled by default.
	return c != nil && c.Enabled != nil && *c.Enabled
}

func replaceVersion(pattern, version string) string {
	re := regexp.MustCompile("%.*{.*beat.version.?}")
	return re.ReplaceAllLiteralString(pattern, version)
}

func defaultRum(beatVersion string) *rumConfig {
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
		beatVersion:         beatVersion,
	}
}

func defaultConfig(beatVersion string) *Config {
	metricsEnabled := true
	pipelineEnabled, pipelineOverwrite := false, true
	return &Config{
		Host:                net.JoinHostPort("localhost", defaultPort),
		MaxUnzippedSize:     30 * 1024 * 1024, // 30mb
		MaxHeaderSize:       1 * 1024 * 1024,  // 1mb
		ConcurrentRequests:  5,
		MaxConnections:      0, // unlimited
		MaxRequestQueueTime: 2 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		MaxEventSize:        300 * 1024, // 300 kb
		ShutdownTimeout:     5 * time.Second,
		SecretToken:         "",
		AugmentEnabled:      true,
		Expvar: &ExpvarConfig{
			Enabled: new(bool),
			Url:     "/debug/vars",
		},
		Metrics: &metricsConfig{
			Enabled: &metricsEnabled,
		},
		FrontendConfig: defaultRum(beatVersion),
		RumConfig:      defaultRum(beatVersion),
		Register: &registerConfig{
			Ingest: &ingestConfig{
				Pipeline: &pipelineConfig{
					Enabled:   &pipelineEnabled,
					Overwrite: &pipelineOverwrite,
					Path:      filepath.Join("ingest", "pipeline", "definition.json"),
				}},
		},
	}
}
