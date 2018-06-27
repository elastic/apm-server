package metric

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations = monitoring.NewInt(metricMetrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": docType}
)

type sample interface {
	transform(common.MapStr) error
}

type metric struct {
	samples   []sample
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
		samples := common.MapStr{}
		for _, sample := range metric.samples {
			if err := sample.transform(samples); err != nil {
				logp.NewLogger("transform").Warnf("failed to transform sample %#v", sample)
				continue
			}
		}
		context := m.NewContext(&pa.Service, pa.Process, pa.System, nil).Transform(nil)
		if metric.tags != nil {
			context["tags"] = metric.tags
		}
		ev := beat.Event{
			Fields: common.MapStr{
				"processor": processorEntry,
				"context":   context,
				"metric":    samples,
			},
			Timestamp: metric.timestamp,
		}
		events = append(events, ev)
	}
	return events
}

type metricDecoder struct {
	*utility.ManualDecoder
}

func DecodePayload(raw map[string]interface{}) (*Payload, error) {
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

func (md *metricDecoder) decodeSamples(input interface{}) []sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	samples := make([]sample, len(raw))
	i := 0
	for name, s := range raw {
		sampleMap, ok := s.(map[string]interface{})
		if !ok {
			md.Err = fmt.Errorf("invalid sample: %s: %s", name, s)
			return nil
		}

		sampleType, ok := sampleMap["type"]
		if !ok {
			md.Err = fmt.Errorf("missing sample type: %s: %s", name, s)
			return nil
		}

		var sample sample
		switch sampleType {
		case "counter":
			sample = md.decodeCounter(name, sampleMap)
		case "gauge":
			sample = md.decodeGauge(name, sampleMap)
		case "summary":
			sample = md.decodeSummary(name, sampleMap)
		}
		if md.Err != nil {
			return nil
		}
		samples[i] = sample
		i++
	}
	return samples
}
