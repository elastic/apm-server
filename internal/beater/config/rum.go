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

package config

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-ucfg"
)

const (
	allowAllOrigins            = "*"
	defaultExcludeFromGrouping = "^/webpack"
	defaultLibraryPattern      = "node_modules|bower_components|~"
	defaultSourcemapTimeout    = 5 * time.Second
	// defaultMaxSourceMapSizeBytes is the default max decompressed source map size (100 MB).
	// 100 MB is ~500x the default max payload size allowed by Kibana:
	// https://www.elastic.co/docs/api/doc/kibana/v9/operation/operation-uploadsourcemap
	defaultMaxSourceMapSizeStr   = "500MiB"
	defaultMaxSourceMapSizeBytes = 500 * 1024 * 1024
)

var (
	errParseSourceMapMaxSize = errors.New("error parsing sourcemap max size")
)

// RumConfig holds config information related to the RUM endpoint
type RumConfig struct {
	Enabled             bool                `config:"enabled"`
	AllowOrigins        []string            `config:"allow_origins"`
	AllowHeaders        []string            `config:"allow_headers"`
	ResponseHeaders     map[string][]string `config:"response_headers"`
	LibraryPattern      string              `config:"library_pattern"`
	ExcludeFromGrouping string              `config:"exclude_from_grouping"`
	SourceMapping       SourceMapping       `config:"source_mapping"`
}

// SourceMapping holds sourcemap config information
type SourceMapping struct {
	Enabled  bool                  `config:"enabled"`
	ESConfig *elasticsearch.Config `config:"elasticsearch"`
	Timeout  time.Duration         `config:"timeout" validate:"positive"`

	// MaxSourcemapSize is the user-configured max size for decompressed source maps.
	MaxSourcemapSize string `config:"max_sourcemap_size"`
	// MaxSourceMapSizeParsed is the parsed byte value of MaxSourcemapSize.
	MaxSourceMapSizeParsed uint64

	esOverrideConfigured bool
	es                   *config.C
}

func (c *RumConfig) setup(log *logp.Logger, outputESCfg *config.C) error {
	if !c.Enabled {
		return nil
	}

	if _, err := regexp.Compile(c.LibraryPattern); err != nil {
		return fmt.Errorf("invalid regex for `library_pattern`: %w", err)
	}
	if _, err := regexp.Compile(c.ExcludeFromGrouping); err != nil {
		return fmt.Errorf("invalid regex for `exclude_from_grouping`: %w", err)
	}

	if outputESCfg == nil {
		log.Info("Unable to determine sourcemap storage, sourcemaps will not be applied")
		return nil
	}

	// Unpack the output elasticsearch config first
	if err := outputESCfg.Unpack(c.SourceMapping.ESConfig); err != nil {
		return fmt.Errorf("unpacking Elasticsearch output config into Sourcemap config: %w", err)
	}

	// SourceMapping ES config not configured, use the main one and return early
	if c.SourceMapping.es == nil {
		log.Info("Using default sourcemap Elasticsearch config")
		return nil
	}

	// Empty out credential fields before merging if credentials are provided in SourceMapping ES config
	if c.SourceMapping.es.HasField("api_key") || c.SourceMapping.es.HasField("username") || c.SourceMapping.es.HasField("password") {
		c.SourceMapping.ESConfig.APIKey = ""
		c.SourceMapping.ESConfig.Username = ""
		c.SourceMapping.ESConfig.Password = ""
	}

	// Unpack the SourceMapping ES config on top of the output elasticsearch config
	if err := c.SourceMapping.es.Unpack(c.SourceMapping.ESConfig); err != nil {
		return fmt.Errorf("unpacking Elasticsearch sourcemap config into Sourcemap config: %w", err)
	}

	c.SourceMapping.es = nil

	return nil
}

func (s *SourceMapping) Unpack(inp *config.C) error {
	type underlyingSourceMapping SourceMapping
	cfg := underlyingSourceMapping(defaultSourcemapping())
	if err := inp.Unpack(&cfg); err != nil {
		return fmt.Errorf("error unpacking sourcemapping config: %w", err)
	}

	parsed, err := parseUserDefinedBytesConfig(cfg.MaxSourcemapSize)
	if err != nil {
		return fmt.Errorf("%w - %q : %w", errParseSourceMapMaxSize, cfg.MaxSourcemapSize, err)
	}
	cfg.MaxSourceMapSizeParsed = parsed

	*s = SourceMapping(cfg)

	s.esOverrideConfigured = inp.HasField("elasticsearch")
	var (
		e ucfg.Error
	)
	if s.es, err = inp.Child("elasticsearch", -1); err != nil && (!errors.As(err, &e) || e.Reason() != ucfg.ErrMissing) {
		return fmt.Errorf("error storing sourcemap elasticsearch config: %w", err)
	}
	return nil
}

// parseUserDefinedBytesConfig parses the provided size configuration to bytes.
func parseUserDefinedBytesConfig(configValue string) (uint64, error) {
	if configValue == "" {
		return 0, fmt.Errorf("configuration value is empty")
	}

	bytes, err := humanize.ParseBytes(configValue)
	if err != nil {
		return 0, err
	}
	if bytes == 0 {
		return 0, fmt.Errorf("parsed size must be positive")
	}
	return bytes, nil
}

func defaultSourcemapping() SourceMapping {
	cfg := SourceMapping{
		Enabled:          true,
		ESConfig:         elasticsearch.DefaultConfig(),
		Timeout:          defaultSourcemapTimeout,
		MaxSourcemapSize: defaultMaxSourceMapSizeStr,
	}
	parsed, err := humanize.ParseBytes(cfg.MaxSourcemapSize)
	if err != nil {
		panic(err)
	}
	cfg.MaxSourceMapSizeParsed = parsed
	return cfg
}

func defaultRum() RumConfig {
	return RumConfig{
		AllowOrigins:        []string{allowAllOrigins},
		AllowHeaders:        []string{},
		SourceMapping:       defaultSourcemapping(),
		LibraryPattern:      defaultLibraryPattern,
		ExcludeFromGrouping: defaultExcludeFromGrouping,
	}
}
