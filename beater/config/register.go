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
	"path/filepath"

	"github.com/elastic/beats/v7/libbeat/paths"
)

const (
	defaultAPMPipeline = "apm"
)

// RegisterConfig holds ingest config information
type RegisterConfig struct {
	Ingest IngestConfig `config:"ingest"`
}

// IngestConfig holds config pipeline ingest information
type IngestConfig struct {
	Pipeline PipelineConfig `config:"pipeline"`
}

// PipelineConfig holds config information about registering ingest pipelines
type PipelineConfig struct {
	Enabled   bool `config:"enabled"`
	Overwrite bool `config:"overwrite"`
	Path      string
}

func defaultRegisterConfig() RegisterConfig {
	return RegisterConfig{
		Ingest: IngestConfig{
			Pipeline: PipelineConfig{
				Enabled: true,
				Path: paths.Resolve(
					paths.Home, filepath.Join("ingest", "pipeline", "definition.json"),
				),
			},
		},
	}
}
