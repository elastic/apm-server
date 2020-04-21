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

package modeldecoder

import (
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/sourcemap/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

// SourcemapSchema is the compiled JSON Schema for validating sourcemaps.
//
// TODO(axw) make DecodeSourcemap validate against SourcemapSchema, and unexpose this.
// This will require changes to processor/asset/sourcemap.
var SourcemapSchema = validation.CreateSchema(schema.PayloadSchema, "sourcemap")

// DecodeSourcemap decodes a sourcemap.
func DecodeSourcemap(raw map[string]interface{}) (transform.Transformable, error) {
	decoder := utility.ManualDecoder{}
	pa := model.Sourcemap{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	return &pa, decoder.Err
}
