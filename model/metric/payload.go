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
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/metric/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/validation"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "metric"
	docType       = "metric"
)

var (
	transformations = monitoring.NewInt(Metrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": docType}

	Metrics = monitoring.Default.NewRegistry("apm-server.processor.metric", monitoring.PublishExpvar)
)

var cachedSchema = validation.CreateSchema(schema.PayloadSchema, processorName)

func PayloadSchema() *jsonschema.Schema {
	return cachedSchema
}

func DecodePayload(raw map[string]interface{}) ([]transform.Eventable, error) {
	if raw == nil {
		return nil, nil
	}

	decoder := metricDecoder{&utility.ManualDecoder{}}
	metricsIntfArr := decoder.InterfaceArr(raw, "metrics")
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	metrics := make([]transform.Eventable, len(metricsIntfArr))
	for idx, metricData := range metricsIntfArr {
		metrics[idx] = decoder.decodeMetric(metricData)
		if decoder.Err != nil {
			return nil, decoder.Err
		}
	}
	return metrics, decoder.Err
}

func (md *metricDecoder) decodeMetric(input interface{}) *metric {
	if input == nil {
		md.Err = errors.New("no data for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for metric event")
		return nil
	}

	metric := metric{
		samples:   md.decodeSamples(raw["samples"]),
		timestamp: md.TimeRFC3339WithDefault(raw, "timestamp"),
	}
	if tags := utility.Prune(md.MapStr(raw, "tags")); len(tags) > 0 {
		metric.tags = tags
	}
	return &metric
}

func (md *metricDecoder) decodeSamples(input interface{}) []*sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	samples := make([]*sample, len(raw))
	i := 0
	for name, s := range raw {
		if s == nil {
			continue
		}
		sampleMap, ok := s.(map[string]interface{})
		if !ok {
			md.Err = fmt.Errorf("invalid sample: %s: %s", name, s)
			return nil
		}

		samples[i] = &sample{
			name:  name,
			value: md.Float64(sampleMap, "value"),
		}
		if md.Err != nil {
			return nil
		}
		i++
	}
	return samples
}
