package metric

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

// assertMetricsMatch is an equality test for a metric as sample order is not important
func assertMetricsMatch(t *testing.T, expected, actual metric) bool {
	samplesMatch := assert.ElementsMatch(t, expected.samples, actual.samples)
	expected.samples = nil
	actual.samples = nil
	nonSamplesMatch := assert.Equal(t, expected, actual)

	return assert.True(t, samplesMatch && nonSamplesMatch,
		fmt.Sprintf("metrics mismatch\nexpected:%#v\n   actual:%#v", expected, actual))
}

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	pid, ip := 1, "127.0.0.1"
	unit := "foos"
	for _, test := range []struct {
		input map[string]interface{}
		err   error
		p     *Payload
	}{
		{input: nil, err: nil, p: nil},
		{
			input: map[string]interface{}{"service": 123},
			err:   errors.New("Invalid type for service"),
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   errors.New("Invalid type for system"),
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   errors.New("Invalid type for process"),
		},
		{
			input: map[string]interface{}{},
			err:   nil,
			p: &Payload{
				Service: m.Service{}, System: nil,
				Process: nil, Metrics: []*metric{},
			},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples":   map[string]interface{}{},
					},
				},
			},
			err: nil,
			p: &Payload{
				Service: m.Service{
					Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
				Metrics: []*metric{
					{
						samples:   []sample{},
						tags:      nil,
						timestamp: timestampParsed,
					},
				},
			},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.counter": map[string]interface{}{
								"type":  "counter",
								"value": "foo",
							},
						},
					},
				},
			},
			err: errors.New("Error fetching field"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.gauge": map[string]interface{}{
								"type":  "gauge",
								"value": "foo",
							},
						},
					},
				},
			},
			err: errors.New("Error fetching field"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"empty.metric": map[string]interface{}{},
						},
					},
				},
			},
			err: errors.New("missing sample type"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.key.metric": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			err: errors.New("missing sample type"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"tags": map[string]interface{}{
							"a.tag": "a.tag.value",
						},
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"a.counter": map[string]interface{}{
								"type":  "counter",
								"value": json.Number("612"),
								"unit":  unit,
							},
							"some.gauge": map[string]interface{}{
								"type":  "gauge",
								"value": json.Number("9.16"),
							},
						},
					},
				},
			},
			err: nil,
			p: &Payload{
				Service: m.Service{Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
				Metrics: []*metric{
					{
						samples: []sample{
							&gauge{
								name:  "some.gauge",
								value: 9.16,
							},
							&counter{
								name:  "a.counter",
								count: 612,
								unit:  &unit,
							},
						},
						tags: common.MapStr{
							"a.tag": "a.tag.value",
						},
						timestamp: timestampParsed,
					},
				},
			},
		},
	} {
		payload, err := DecodePayload(test.input)

		// compare metrics separately as they may be ordered differently
		if test.p != nil && test.p.Metrics != nil {
			for i := range test.p.Metrics {
				if test.p.Metrics[i] == nil {
					continue
				}
				want := *test.p.Metrics[i]
				got := *payload.Metrics[i]
				assertMetricsMatch(t, want, got)
			}
			// leave everything else in payload for comparison
			test.p.Metrics = nil
			payload.Metrics = nil
		}
		// compare remaining payload
		assert.Equal(t, test.p, payload)
		if test.err != nil {
			assert.Error(t, err)
		}
	}
}

func TestPayloadTransform(t *testing.T) {
	svc := m.Service{Name: "myservice"}
	timestamp := time.Now()
	unit := "bytes"

	tests := []struct {
		Payload Payload
		Output  []common.MapStr
		Msg     string
	}{
		{
			Payload: Payload{Service: svc, Metrics: []*metric{}},
			Output:  nil,
			Msg:     "Empty metric Array",
		},
		{
			Payload: Payload{Service: svc, Metrics: []*metric{{timestamp: timestamp}}},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myservice",
						},
					},
					"metric":    common.MapStr{},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Payload: Payload{
				Service: svc,
				Metrics: []*metric{
					{
						tags:      common.MapStr{"a.tag": "a.tag.value"},
						timestamp: timestamp,
						samples: []sample{
							&counter{
								name:  "a.counter",
								count: 612,
							},
							&gauge{
								name:  "some.gauge",
								value: 9.16,
								unit:  &unit,
							},
						},
					},
				},
			},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"name":  "myservice",
							"agent": common.MapStr{"name": "", "version": ""},
						},
						"tags": common.MapStr{
							"a.tag": "a.tag.value",
						},
					},
					"metric": common.MapStr{
						"a.counter":  common.MapStr{"value": float64(612), "type": "counter"},
						"some.gauge": common.MapStr{"value": float64(9.16), "type": "gauge", "unit": unit},
					},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with valid metric.",
		},
	}

	for idx, test := range tests {
		conf := config.Config{}
		outputEvents := test.Payload.Transform(conf)
		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
