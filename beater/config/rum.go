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

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
)

const (
	allowAllOrigins                 = "*"
	defaultExcludeFromGrouping      = "^/webpack"
	defaultLibraryPattern           = "node_modules|bower_components|~"
	defaultSourcemapCacheExpiration = 5 * time.Minute
	defaultSourcemapIndexPattern    = "apm-*-sourcemap*"
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
	Cache        Cache                 `config:"cache"`
	Enabled      bool                  `config:"enabled"`
	IndexPattern string                `config:"index_pattern"`
	ESConfig     *elasticsearch.Config `config:"elasticsearch"`
	Metadata     []SourceMapMetadata   `config:"metadata"`
	esConfigured bool
}

func (c *RumConfig) setup(log *logp.Logger, dataStreamsEnabled bool, outputESCfg *common.Config) error {
	if !c.Enabled {
		return nil
	}

	if _, err := regexp.Compile(c.LibraryPattern); err != nil {
		return errors.Wrapf(err, "Invalid regex for `library_pattern`: ")
	}
	if _, err := regexp.Compile(c.ExcludeFromGrouping); err != nil {
		return errors.Wrapf(err, "Invalid regex for `exclude_from_grouping`: ")
	}

	if c.SourceMapping.esConfigured && len(c.SourceMapping.Metadata) > 0 {
		return errors.New("configuring both source_mapping.elasticsearch and sourcemapping.source_maps not allowed")
	}

	// No need to unpack the ESConfig if SourceMapMetadata exist
	if len(c.SourceMapping.Metadata) > 0 {
		return nil
	}

	// fall back to elasticsearch output configuration for sourcemap storage if possible
	if outputESCfg == nil {
		log.Info("Unable to determine sourcemap storage, sourcemaps will not be applied")
		return nil
	}
	log.Info("Falling back to elasticsearch output for sourcemap storage")
	if err := outputESCfg.Unpack(c.SourceMapping.ESConfig); err != nil {
		return errors.Wrap(err, "unpacking Elasticsearch config into Sourcemap config")
	}
	return nil
}

func (s *SourceMapping) Unpack(inp *common.Config) error {
	type underlyingSourceMapping SourceMapping
	if err := inp.Unpack((*underlyingSourceMapping)(s)); err != nil {
		return errors.Wrap(err, "error unpacking sourcemapping config")
	}
	s.esConfigured = inp.HasField("elasticsearch")
	return nil
}

func defaultSourcemapping() SourceMapping {
	return SourceMapping{
		Enabled:      true,
		Cache:        Cache{Expiration: defaultSourcemapCacheExpiration},
		IndexPattern: defaultSourcemapIndexPattern,
		ESConfig:     elasticsearch.DefaultConfig(),
		Metadata:     []SourceMapMetadata{},
	}
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
