package package_tests

import (
	"encoding/json"
	"testing"

	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/tests"
)

var (
	procSetup = tests.ProcessorSetup{
		Proc:            metric.NewProcessor(),
		FullPayloadPath: "../testdata/metric/payload.json",
		TemplatePaths:   []string{"../_meta/fields.yml", "../../../_meta/fields.common.yml"},
	}
)

func TestInvalidPayloads(t *testing.T) {
	type obj = map[string]interface{}
	type val = []interface{}

	validCounter := obj{"type": "counter", "value": json.Number("1.0")}
	validGauge := obj{"type": "gauge", "value": json.Number("1.0")}
	// every payload needs a timestamp, these reduce the clutter
	tsk, ts := "timestamp", "2017-05-30T18:53:42.281Z"

	payloadData := []tests.SchemaTestData{
		{
			Key: "metrics",
			Valid: val{
				val{obj{tsk: ts, "samples": obj{"valid-counter": validCounter}}},
				val{obj{tsk: ts, "samples": obj{"valid-gauge": validGauge}}},
				val{obj{tsk: ts, "samples": obj{"valid-gauge": obj{"type": "gauge", "value": json.Number("1.0"), "unit": "foos"}}}},
				val{obj{tsk: ts, "samples": obj{"valid-gauge": obj{"type": "gauge", "value": json.Number("1.0"), "unit": nil}}}},
			},
			Invalid: []tests.Invalid{
				{
					Msg: "properties/metrics/items/required",
					Values: val{
						val{obj{tsk: ts}},
					},
				},
				{
					Msg: "properties/metrics/items/properties/samples/type",
					Values: val{
						val{obj{tsk: ts, "samples": nil}},
						val{obj{tsk: ts, "samples": "samples-as-string"}},
						val{obj{tsk: ts, "samples": val{"samples-as-array"}}},
					},
				},
				{
					Msg: "properties/metrics/items/properties/samples/patternproperties",
					Values: val{
						val{obj{tsk: ts, "samples": obj{"valid-key": nil}}},
						val{obj{tsk: ts, "samples": obj{"no-type": obj{"value": 17}}}},
						val{obj{tsk: ts, "samples": obj{"invalid-type": obj{"type": "foo", "value": 17}}}},
						val{obj{tsk: ts, "samples": obj{"nil-type": obj{"type": nil, "value": "foo"}}}},
						val{obj{tsk: ts, "samples": obj{"nil-counter-value": obj{"type": "counter", "value": nil}}}},
						val{obj{tsk: ts, "samples": obj{"nil-gauge-value": obj{"type": "gauge", "value": nil}}}},
						val{obj{tsk: ts, "samples": obj{"string-counter": obj{"type": "counter", "value": "foo"}}}},
						val{obj{tsk: ts, "samples": obj{"string-gauge": obj{"type": "gauge", "value": "foo"}}}},
					},
				},
				{
					Msg: "properties/metrics/items/properties/samples/additionalproperties",
					Values: val{
						val{obj{tsk: ts, "samples": obj{"metric\"key\"_quotes": validCounter}}},
						val{obj{tsk: ts, "samples": obj{"metric-*-key-star": validGauge}}},
					},
				},
				{
					Msg: "properties/quantiles/items/items/0/minimum",
					Values: val{
						val{obj{tsk: ts, "samples": obj{
							"min-exceeded": obj{
								"type":      "summary",
								"count":     1,
								"sum":       1,
								"quantiles": val{val{0, 6.12}, val{-0.5, 10}, val{0.5, 10}}}}},
						},
					},
				},
				{
					Msg: "properties/quantiles/items/items/0/maximum",
					Values: val{
						val{obj{tsk: ts, "samples": obj{
							"max-exceeded": obj{
								"type":      "summary",
								"count":     1,
								"sum":       1,
								"quantiles": val{val{0, 6.12}, val{100, 10}, val{0.5, 10}}}}},
						},
					},
				},
			},
		},
	}
	procSetup.DataValidation(t, payloadData)
}
