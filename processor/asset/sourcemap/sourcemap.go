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
	parser "github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/metadata"
	sm "github.com/elastic/apm-server/model/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/validation"
)

const eventName = "sourcemap"

var (
	Processor = &sourcemapProcessor{
		PayloadSchema: sm.PayloadSchema(),
		DecodingCount: monitoring.NewInt(sm.Metrics, "decoding.count"),
		DecodingError: monitoring.NewInt(sm.Metrics, "decoding.errors"),
		ValidateCount: monitoring.NewInt(sm.Metrics, "validation.count"),
		ValidateError: monitoring.NewInt(sm.Metrics, "validation.errors"),
	}
)

type sourcemapProcessor struct {
	PayloadKey    string
	EventDecoder  decoder.EventDecoder
	PayloadSchema *jsonschema.Schema
	DecodingCount *monitoring.Int
	DecodingError *monitoring.Int
	ValidateCount *monitoring.Int
	ValidateError *monitoring.Int
}

func (p *sourcemapProcessor) Name() string {
	return eventName
}

func (p *sourcemapProcessor) Decode(raw map[string]interface{}) (*metadata.Metadata, []transform.Transformable, error) {
	p.DecodingCount.Inc()
	transformable, err := sm.DecodeSourcemap(raw)
	if err != nil {
		p.DecodingError.Inc()
		return nil, nil, err
	}

	return &metadata.Metadata{}, []transform.Transformable{transformable}, err
}

func (p *sourcemapProcessor) Validate(raw map[string]interface{}) error {
	p.ValidateCount.Inc()

	smap, ok := raw["sourcemap"].(string)
	if !ok {
		if s := raw["sourcemap"]; s == nil {
			return errors.New(`missing properties: "sourcemap", expected sourcemap to be sent as string, but got null`)
		} else {
			return errors.New("sourcemap not in expected format")
		}
	}

	_, err := parser.Parse("", []byte(smap))
	if err != nil {
		return errors.Wrap(err, "error validating sourcemap")
	}

	err = validation.Validate(raw, p.PayloadSchema)
	if err != nil {
		p.ValidateError.Inc()
	}
	return err
}
