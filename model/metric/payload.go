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
	"time"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/model"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metric/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
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

type sample struct {
	name  string
	value float64
	unit  *string
}

type metric struct {
	samples   []*sample
	tags      common.MapStr
	timestamp time.Time
}

type Payload struct {
	Process *m.Process
	Service m.Service
	System  *m.System
	Metrics []*metric
}

func (pa *Payload) Transform(config.Config) []beat.Event {
	transformations.Inc()
	if pa == nil {
		return nil
	}

	var events []beat.Event
	for _, metric := range pa.Metrics {
		fields := common.MapStr{}
		for _, sample := range metric.samples {
			if _, err := fields.Put(sample.name, sample.value); err != nil {
				logp.NewLogger("transform").Warnf("failed to transform sample %#v", sample)
				continue
			}
		}
		context := m.NewContext(&pa.Service, pa.Process, pa.System, nil).Transform(nil)
		if metric.tags != nil {
			context["tags"] = metric.tags
		}
		fields["context"] = context
		fields["processor"] = processorEntry

		ev := beat.Event{
			Fields:    fields,
			Timestamp: metric.timestamp,
		}
		events = append(events, ev)
	}
	return events
}

type metricDecoder struct {
	*utility.ManualDecoder
}

func DecodePayload(raw map[string]interface{}) (model.Payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := &Payload{}

	service, err := m.DecodeService(raw["service"], nil)
	if service != nil {
		pa.Service = *service
	}
	pa.System, err = m.DecodeSystem(raw["system"], err)
	pa.Process, err = m.DecodeProcess(raw["process"], err)
	if err != nil {
		return nil, err
	}

	decoder := metricDecoder{&utility.ManualDecoder{}}
	metrics := decoder.InterfaceArr(raw, "metrics")
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	pa.Metrics = make([]*metric, len(metrics))
	for idx, metricData := range metrics {
		pa.Metrics[idx] = decoder.decodeMetric(metricData)
		if decoder.Err != nil {
			return nil, decoder.Err
		}
	}
	return pa, decoder.Err
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
			unit:  md.StringPtr(sampleMap, "unit"),
		}
		if md.Err != nil {
			return nil
		}
		i++
	}
	return samples
}
