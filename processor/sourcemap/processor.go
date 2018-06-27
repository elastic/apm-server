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
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/processor/sourcemap/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "sourcemap"
	smapDocType   = "sourcemap"
)

var (
	sourcemapUploadMetrics = monitoring.Default.NewRegistry("apm-server.processor.sourcemap", monitoring.PublishExpvar)
	validationCount        = monitoring.NewInt(sourcemapUploadMetrics, "validation.count")
	validationError        = monitoring.NewInt(sourcemapUploadMetrics, "validation.errors")
	decodingCount          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.count")
	decodingError          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.errors")
)

var loadedSchema = pr.CreateSchema(schema.PayloadSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: loadedSchema}
}

func (p *processor) Name() string {
	return processorName
}

type processor struct {
	schema *jsonschema.Schema
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()

	smap, ok := raw["sourcemap"].(string)
	if !ok {
		if s, _ := raw["sourcemap"]; s == nil {
			return errors.New(`missing properties: "sourcemap", expected sourcemap to be sent as string, but got null`)
		} else {
			return errors.New("sourcemap not in expected format")
		}
	}

	_, err := parser.Parse("", []byte(smap))
	if err != nil {
		return errors.New(fmt.Sprintf("Error validating sourcemap: %v", err))
	}

	err = pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Decode(raw map[string]interface{}) (pr.Payload, error) {
	decodingCount.Inc()

	decoder := utility.ManualDecoder{}
	pa := Payload{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	if decoder.Err != nil {
		decodingError.Inc()
		return nil, decoder.Err
	}
	return &pa, nil
}
