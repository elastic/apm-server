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
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/paths"
)

const (
	// DefaultPort of APM Server
	DefaultPort = "8200"

	defaultAPMPipeline = "apm"
)

type Config struct {
	Host                string                  `config:"host"`
	MaxHeaderSize       int                     `config:"max_header_size"`
	IdleTimeout         time.Duration           `config:"idle_timeout"`
	ReadTimeout         time.Duration           `config:"read_timeout"`
	WriteTimeout        time.Duration           `config:"write_timeout"`
	MaxEventSize        int                     `config:"max_event_size"`
	ShutdownTimeout     time.Duration           `config:"shutdown_timeout"`
	SecretToken         string                  `config:"secret_token"`
	TLS                 *tlscommon.ServerConfig `config:"ssl"`
	MaxConnections      int                     `config:"max_connections"`
	Expvar              *ExpvarConfig           `config:"expvar"`
	AugmentEnabled      bool                    `config:"capture_personal_data"`
	SelfInstrumentation *InstrumentationConfig  `config:"instrumentation"`
	RumConfig           *rumConfig              `config:"rum"`
	Register            *registerConfig         `config:"register"`
	Mode                Mode                    `config:"mode"`
	Kibana              *common.Config          `config:"kibana"`

	pipeline string
}

type ExpvarConfig struct {
	Enabled *bool  `config:"enabled"`
	Url     string `config:"url"`
}

type rumConfig struct {
	Enabled             *bool          `config:"enabled"`
	EventRate           *eventRate     `config:"event_rate"`
	AllowOrigins        []string       `config:"allow_origins"`
	LibraryPattern      string         `config:"library_pattern"`
	ExcludeFromGrouping string         `config:"exclude_from_grouping"`
	SourceMapping       *SourceMapping `config:"source_mapping"`

	beatVersion string
}

type eventRate struct {
	Limit   int `config:"limit"`
	LruSize int `config:"lru_size"`
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

type InstrumentationConfig struct {
	Enabled     *bool   `config:"enabled"`
	Environment *string `config:"environment"`
	Hosts       urls    `config:"hosts" validate:"nonzero"`
	SecretToken string  `config:"secret_token"`
}

//Environment enumerates the APM Server env
type Mode uint8

const (
	ModeProduction Mode = iota
	ModeExperimental
)

func (m *Mode) Unpack(s string) error {
	if strings.ToLower(s) == "experimental" {
		*m = ModeExperimental
		return nil
	}
	*m = ModeProduction
	return nil
}

func newConfig(version string, ucfg *common.Config) (*Config, error) {
	c := defaultConfig(version)

	if ucfg.HasField("ssl") {
		ssl, err := ucfg.Child("ssl", -1)
		if err != nil {
			return nil, err
		}
		if !ssl.HasField("client_authentication") {
			if err := ucfg.SetString("ssl.client_authentication", -1, "optional"); err != nil {
				return nil, err
			}
		}
	}

	if err := ucfg.Unpack(c); err != nil {
		return nil, errors.Wrap(err, "Error processing configuration")
	}

	if c.RumConfig.isEnabled() {
		if _, err := regexp.Compile(c.RumConfig.LibraryPattern); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `library_pattern`: %v", err.Error()))
		}
		if _, err := regexp.Compile(c.RumConfig.ExcludeFromGrouping); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `exclude_from_grouping`: %v", err.Error()))
		}
	}

	return c, nil
}

func (c *Config) setSmapElasticsearch(esConfig *common.Config) {
	if c != nil && c.RumConfig.isEnabled() && c.RumConfig.SourceMapping != nil {
		c.RumConfig.SourceMapping.EsConfig = esConfig
	}
}

func (c *ExpvarConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *rumConfig) isEnabled() bool {
	return c != nil && (c.Enabled != nil && *c.Enabled)
}

func (s *SourceMapping) isSetup() bool {
	return s != nil && (s.EsConfig != nil)
}

func (c *pipelineConfig) isEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

func (c *pipelineConfig) shouldOverwrite() bool {
	return c != nil && (c.Overwrite == nil || *c.Overwrite)
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
	re := regexp.MustCompile("%.*{.*observer.version.?}")
	return re.ReplaceAllLiteralString(pattern, version)
}

func defaultRum(beatVersion string) *rumConfig {
	return &rumConfig{
		EventRate: &eventRate{
			Limit:   300,
			LruSize: 1000,
		},
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
	pipeline := true
	return &Config{
		Host:            net.JoinHostPort("localhost", DefaultPort),
		MaxHeaderSize:   1 * 1024 * 1024, // 1mb
		MaxConnections:  0,               // unlimited
		IdleTimeout:     45 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		MaxEventSize:    300 * 1024, // 300 kb
		ShutdownTimeout: 5 * time.Second,
		SecretToken:     "",
		AugmentEnabled:  true,
		Expvar: &ExpvarConfig{
			Enabled: new(bool),
			Url:     "/debug/vars",
		},
		RumConfig: &rumConfig{
			EventRate: &eventRate{
				Limit:   300,
				LruSize: 1000,
			},
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
		},
		Register: &registerConfig{
			Ingest: &ingestConfig{
				Pipeline: &pipelineConfig{
					Enabled:   &pipeline,
					Overwrite: &pipeline,
					Path: paths.Resolve(paths.Home,
						filepath.Join("ingest", "pipeline", "definition.json")),
				}},
		},
		Mode:     ModeProduction,
		Kibana:   common.MustNewConfigFrom(map[string]interface{}{"enabled": "false"}),
		pipeline: defaultAPMPipeline,
	}
}
