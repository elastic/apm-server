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

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/sourcemap/generated/schema"
	smap "github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	sourcemapCounter = monitoring.NewInt(Metrics, "counter")
	processorEntry   = common.MapStr{"name": processorName, "event": smapDocType}
)

const (
	processorName = "sourcemap"
	smapDocType   = "sourcemap"
)

var (
	Metrics = monitoring.Default.NewRegistry("apm-server.processor.sourcemap", monitoring.PublishExpvar)
)

var cachedSchema = validation.CreateSchema(schema.PayloadSchema, processorName)

func PayloadSchema() *jsonschema.Schema {
	return cachedSchema
}

type Payload struct {
	ServiceName    string
	ServiceVersion string
	Sourcemap      string
	BundleFilepath string
}

func (pa *Payload) Events(tctx *transform.Context) []beat.Event {
	sourcemapCounter.Add(1)
	if pa == nil {
		return nil
	}

	if tctx.Config.SmapMapper == nil {
		logp.NewLogger("sourcemap").Error("Sourcemap Accessor is nil, cache cannot be invalidated.")
	} else {
		tctx.Config.SmapMapper.NewSourcemapAdded(smap.Id{
			ServiceName:    pa.ServiceName,
			ServiceVersion: pa.ServiceVersion,
			Path:           pa.BundleFilepath,
		})
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor": processorEntry,
			smapDocType: common.MapStr{
				"bundle_filepath": pa.BundleFilepath,
				"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
				"sourcemap":       pa.Sourcemap,
			},
		},
		Timestamp: time.Now(),
	}
	return []beat.Event{ev}
}

func DecodePayload(raw map[string]interface{}) ([]transform.Eventable, error) {
	decoder := utility.ManualDecoder{}
	pa := Payload{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	return []transform.Eventable{&pa}, decoder.Err
}
