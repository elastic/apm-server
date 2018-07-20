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

package processor

import (
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/monitoring"
)

type Processor interface {
	Validate(map[string]interface{}) error
	Decode(map[string]interface{}) (*metadata.Metadata, []transform.Eventable, error)
	Name() string
}

type PayloadDecoder func(map[string]interface{}) ([]transform.Eventable, error)

type PayloadProcessor struct {
	ProcessorName string
	DecodePayload PayloadDecoder
	PayloadSchema *jsonschema.Schema
	DecodingCount *monitoring.Int
	DecodingError *monitoring.Int
	ValidateCount *monitoring.Int
	ValidateError *monitoring.Int
}

func (p *PayloadProcessor) Name() string {
	return p.ProcessorName
}

func (p *PayloadProcessor) Decode(raw map[string]interface{}) (*metadata.Metadata, []transform.Eventable, error) {
	p.DecodingCount.Inc()
	payload, err := p.DecodePayload(raw)
	if err != nil {
		p.DecodingError.Inc()
		return nil, nil, err
	}

	metadata, err := metadata.DecodeMetadata(raw)
	if err != nil {
		p.DecodingError.Inc()
		return nil, nil, err
	}

	return metadata, payload, err
}

func (p *PayloadProcessor) Validate(raw map[string]interface{}) error {
	p.ValidateCount.Inc()
	err := validation.Validate(raw, p.PayloadSchema)
	if err != nil {
		p.ValidateError.Inc()
	}
	return err
}
