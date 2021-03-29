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

package transform

import (
	"context"
	"regexp"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-server/sourcemap"
)

// Transformable is an interface implemented by all top-level model objects for
// translating to beat.Events.
type Transformable interface {
	Transform(context.Context, *Config) []beat.Event
}

// Config holds general transformation configuration.
type Config struct {
	// DataStreams records whether or not data streams are enabled.
	// If true, then data_stream fields should be added to all events.
	DataStreams bool

	RUM RUMConfig
}

// RUMConfig holds RUM-related transformation configuration.
type RUMConfig struct {
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
	SourcemapStore      *sourcemap.Store
	MaxLineLength       uint
}
