package metric

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

var (
	processorEntry = common.MapStr{"name": processorName, "event": docType}
)

type Sample interface {
	Name() string
	Value() interface{}
}

type FloatSample struct {
	name  string
	value json.Number
}

func (c FloatSample) Name() string {
	return c.name
}

func (c FloatSample) Value() interface{} {
	return c.value
}

type Metric struct {
	Samples   []Sample
	Timestamp time.Time
}

type Payload struct {
	Process *m.Process
	Service m.Service
	System  *m.System
	Metrics []Metric
}

func decodeSamples(raw map[string]interface{}) []Sample {
	sl := make([]Sample, 0)

	samples := raw["samples"].(map[string]interface{})
	fmt.Println(samples)
	for name, s := range samples {
		sample := s.(map[string]interface{})
		v, ok := sample["value"]
		if !ok {
			continue
		}
		f, ok := v.(json.Number)
		if !ok {
			continue
		}
		sl = append(sl, FloatSample{
			name:  name,
			value: f,
		})
	}

	return sl
}

func DecodePayload(raw map[string]interface{}) (*Payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := Payload{}

	var err error
	service, err := m.DecodeService(raw["service"], err)
	if service != nil {
		pa.Service = *service
	}
	pa.System, err = m.DecodeSystem(raw["system"], err)
	pa.Process, err = m.DecodeProcess(raw["process"], err)
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	metrics := decoder.InterfaceArr(raw, "metrics")
	for _, m := range metrics {
		raw := m.(map[string]interface{})
		pa.Metrics = append(pa.Metrics, Metric{
			Samples:   decodeSamples(raw),
			Timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
		})
	}
	return &pa, nil
}

func (pa *Payload) Transform(conf config.Config) []beat.Event {
	if pa == nil {
		return nil
	}

	var events []beat.Event
	for _, metric := range pa.Metrics {
		fields := common.MapStr{
			"processor": processorEntry,
			"process":   pa.Process.Transform(),
			"service":   pa.Service.Transform(),
			"system":    pa.System.Transform(),
		}
		for _, sample := range metric.Samples {
			fields[sample.Name()] = sample.Value()
		}
		ev := beat.Event{
			Fields:    fields,
			Timestamp: metric.Timestamp,
		}
		events = append(events, ev)
	}
	return events
}
