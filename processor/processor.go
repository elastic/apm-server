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

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/monitoring"
)

type Processor interface {
	Validate(map[string]interface{}) error
	Decode(map[string]interface{}) (*metadata.Metadata, []transform.Transformable, error)
	Name() string
}

type EventsProcessor struct {
	EventName     string
	PayloadKey    string
	EventDecoder  decoder.EventDecoder
	PayloadSchema *jsonschema.Schema
	DecodingCount *monitoring.Int
	DecodingError *monitoring.Int
	ValidateCount *monitoring.Int
	ValidateError *monitoring.Int
}

func (p *EventsProcessor) Name() string {
	return p.EventName
}

func (p *EventsProcessor) decodePayload(raw map[string]interface{}) ([]transform.Transformable, error) {
	var err error
	decoder := utility.ManualDecoder{}

	rawObjects := decoder.InterfaceArr(raw, p.PayloadKey)

	events := make([]transform.Transformable, len(rawObjects))
	for idx, errData := range rawObjects {
		events[idx], err = p.EventDecoder(errData, err)
	}
	return events, err
}

func (p *EventsProcessor) Decode(raw map[string]interface{}) (*metadata.Metadata, []transform.Transformable, error) {
	p.DecodingCount.Inc()
	payload, err := p.decodePayload(raw)
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

func (p *EventsProcessor) Validate(raw map[string]interface{}) error {
	p.ValidateCount.Inc()
	err := validation.Validate(raw, p.PayloadSchema)
	if err != nil {
		p.ValidateError.Inc()
	}
	return err
}
