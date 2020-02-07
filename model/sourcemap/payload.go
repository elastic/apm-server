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

package sourcemap

import (
	"time"

	"github.com/elastic/apm-server/sourcemap"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model/sourcemap/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

const (
	processorName = "sourcemap"
	smapDocType   = "sourcemap"
)

var (
	Metrics          = monitoring.Default.NewRegistry("apm-server.processor.sourcemap", monitoring.PublishExpvar)
	sourcemapCounter = monitoring.NewInt(Metrics, "counter")

	processorEntry = common.MapStr{"name": processorName, "event": smapDocType}
)

var cachedSchema = validation.CreateSchema(schema.PayloadSchema, processorName)

func PayloadSchema() *jsonschema.Schema {
	return cachedSchema
}

type Sourcemap struct {
	ServiceName    string
	ServiceVersion string
	Sourcemap      string
	BundleFilepath string
}

func (pa *Sourcemap) Transform(store *sourcemap.Store) []beat.Event {
	sourcemapCounter.Inc()
	if pa == nil {
		return nil
	}

	if store == nil {
		logp.NewLogger(logs.Sourcemap).Error("Sourcemap Accessor is nil, cache cannot be invalidated.")
	} else {
		store.Added(pa.ServiceName, pa.ServiceVersion, pa.BundleFilepath)
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor": processorEntry,
			smapDocType: common.MapStr{
				"bundle_filepath": utility.UrlPath(pa.BundleFilepath),
				"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
				"sourcemap":       pa.Sourcemap,
			},
		},
		Timestamp: time.Now(),
	}
	return []beat.Event{ev}
}

func DecodeSourcemap(raw map[string]interface{}) (*Sourcemap, error) {
	decoder := utility.ManualDecoder{}
	pa := Sourcemap{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	return &pa, decoder.Err
}
