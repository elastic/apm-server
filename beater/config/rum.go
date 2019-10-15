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
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/sourcemap"
)

// RumConfig holds config information related to the RUM endpoint
type RumConfig struct {
	Enabled             *bool          `config:"enabled"`
	EventRate           *EventRate     `config:"event_rate"`
	AllowOrigins        []string       `config:"allow_origins"`
	LibraryPattern      string         `config:"library_pattern"`
	ExcludeFromGrouping string         `config:"exclude_from_grouping"`
	SourceMapping       *SourceMapping `config:"source_mapping"`

	BeatVersion string
}

// EventRate holds config information about event rate limiting
type EventRate struct {
	Limit   int `config:"limit"`
	LruSize int `config:"lru_size"`
}

// SourceMapping holds sourecemap config information
type SourceMapping struct {
	Cache        *Cache         `config:"cache"`
	Enabled      *bool          `config:"enabled"`
	IndexPattern string         `config:"index_pattern"`
	EsConfig     *common.Config `config:"elasticsearch"`
	mapper       sourcemap.Mapper
}

func (c *RumConfig) setup(log *logp.Logger, outputESCfg *common.Config) error {
	if !c.IsEnabled() {
		return nil
	}

	if _, err := regexp.Compile(c.LibraryPattern); err != nil {
		return errors.Wrapf(err, "Invalid regex for `library_pattern`: ")
	}
	if _, err := regexp.Compile(c.ExcludeFromGrouping); err != nil {
		return errors.Wrapf(err, "Invalid regex for `exclude_from_grouping`: ")
	}

	if c.SourceMapping == nil || c.SourceMapping.EsConfig != nil {
		return nil
	}

	// fall back to elasticsearch output configuration for sourcemap storage if possible
	if outputESCfg == nil {
		log.Info("Unable to determine sourcemap storage, sourcemaps will not be applied")
		return nil
	}
	log.Info("Falling back to elasticsearch output for sourcemap storage")
	c.SourceMapping.EsConfig = outputESCfg
	return nil
}

// IsEnabled indicates whether RUM endpoint is enabled or not
func (c *RumConfig) IsEnabled() bool {
	return c != nil && (c.Enabled != nil && *c.Enabled)
}

// IsEnabled indicates whether sourcemap handling is enabled or not
func (s *SourceMapping) IsEnabled() bool {
	return s == nil || s.Enabled == nil || *s.Enabled
}

// MemoizedSourcemapMapper creates the sourcemap mapper once and then caches it
func (c *RumConfig) MemoizedSourcemapMapper() (sourcemap.Mapper, error) {
	if !c.IsEnabled() || !c.SourceMapping.IsEnabled() || !c.SourceMapping.isSetup() {
		return nil, nil
	}
	if c.SourceMapping.mapper != nil {
		return c.SourceMapping.mapper, nil
	}

	sourcemapConfig := sourcemap.Config{
		CacheExpiration:     c.SourceMapping.Cache.Expiration,
		ElasticsearchConfig: c.SourceMapping.EsConfig,
		Index:               replaceVersion(c.SourceMapping.IndexPattern, c.BeatVersion),
	}
	mapper, err := sourcemap.NewSmapMapper(sourcemapConfig)
	if err != nil {
		return nil, err
	}
	c.SourceMapping.mapper = mapper
	return c.SourceMapping.mapper, nil
}

func (s *SourceMapping) isSetup() bool {
	return s != nil && (s.EsConfig != nil)
}

func defaultRum(beatVersion string) *RumConfig {
	return &RumConfig{
		EventRate: &EventRate{
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
		BeatVersion:         beatVersion,
	}
}
