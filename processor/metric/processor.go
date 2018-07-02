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

package metric

import (
	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/processor/metric/generated/schema"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "metric"
	docType       = "metric"
)

var (
	metricMetrics   = monitoring.Default.NewRegistry("apm-server.processor.metric", monitoring.PublishExpvar)
	validationCount = monitoring.NewInt(metricMetrics, "validation.count")
	validationError = monitoring.NewInt(metricMetrics, "validation.error")
	decodingCount   = monitoring.NewInt(metricMetrics, "decoding.count")
	decodingError   = monitoring.NewInt(metricMetrics, "decoding.errors")
)

type processor struct {
	schema *jsonschema.Schema
}

var loadedSchema = pr.CreateSchema(schema.PayloadSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: loadedSchema}
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	err := pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Decode(raw map[string]interface{}) (pr.Payload, error) {
	decodingCount.Inc()
	pa, err := DecodePayload(raw)
	if err != nil {
		decodingError.Inc()
		return nil, err
	}
	return pa, nil
}
