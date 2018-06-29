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

func TestAttributesPresenceInMetric(t *testing.T) {
	requiredKeys := tests.NewSet("service", "metrics", "metrics.samples", "metrics.timestamp", "metrics.samples.+.type", "metrics.samples.+.value", "metrics.samples.+.count", "metrics.samples.+.sum")
	procSetup.AttrsPresence(t, requiredKeys, nil)
}

func TestInvalidPayloads(t *testing.T) {
	type obj = map[string]interface{}
	type val = []interface{}

	validCounter := obj{"type": "counter", "value": json.Number("1.0")}
	validGauge := obj{"type": "gauge", "value": json.Number("1.0")}
	// every payload needs a timestamp, these reduce the clutter
	//tsk, ts := "timestamp", "2017-05-30T18:53:42.281Z"

	payloadData := []tests.SchemaTestData{
		{Key: "metrics", Invalid: []tests.Invalid{
			{Msg: "metrics/minitems", Values: val{[]interface{}{}}},
			{Msg: "metrics/type", Values: val{"metrics-string"}}},
		},
		{Key: "metrics.timestamp",
			Valid: val{"2017-05-30T18:53:42.281Z"},
			Invalid: []tests.Invalid{
				{Msg: `timestamp/format`, Values: val{"2017-05-30T18:53Z", "2017-05-30T18:53:27.Z", "2017-05-30T18:53:27a123Z"}},
				{Msg: `timestamp/pattern`, Values: val{"2017-05-30T18:53:27.000+00:20", "2017-05-30T18:53:27ZNOTCORRECT"}}}},
		{Key: "metrics.tags",
			Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
			Invalid: []tests.Invalid{
				{Msg: `tags/type`, Values: val{"tags"}},
				{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
				{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}}}},
		},
		{
			Key: "metrics.samples",
			Valid: val{
				obj{"valid-counter": validCounter},
				obj{"valid-gauge": validGauge},
				obj{"valid-gauge": obj{"type": "gauge", "value": json.Number("1.0"), "unit": "foos"}}},
			Invalid: []tests.Invalid{
				{
					Msg:    "properties/metrics/items/properties/samples/type",
					Values: val{nil, "samples-as-string", "samples-as-array"},
				},
				{
					Msg: "properties/metrics/items/properties/samples/additionalproperties",
					Values: val{
						obj{"metric\"key\"_quotes": validCounter},
						obj{"metric-*-key-star": validGauge},
					},
				},
				{
					Msg: "properties/metrics/items/properties/samples/patternproperties",
					Values: val{
						obj{"invalid-type": obj{"type": "foo", "value": 17}},
						obj{"nil-counter-value": obj{"type": "counter", "value": nil}},
						obj{"nil-gauge-value": obj{"type": "gauge", "value": nil}},
						obj{"string-counter": obj{"type": "counter", "value": "foo"}},
						obj{"string-gauge": obj{"type": "gauge", "value": "foo"}},
						obj{"missing-count": obj{"type": "summary", "sum": 1}},
						obj{"missing-sum": obj{"type": "summary", "count": 1}},
					},
				},
				{
					Msg: "properties/quantiles/items/items/0/minimum",
					Values: val{
						obj{
							"min-exceeded": obj{
								"type":      "summary",
								"count":     1,
								"sum":       1,
								"quantiles": val{val{0, 6.12}, val{-0.5, 10}, val{0.5, 10}}},
						},
					},
				},
				{
					Msg: "properties/quantiles/items/items/0/maximum",
					Values: val{
						obj{
							"max-exceeded": obj{
								"type":      "summary",
								"count":     1,
								"sum":       1,
								"quantiles": val{val{0, 6.12}, val{100, 10}, val{0.5, 10}}},
						},
					},
				},
			},
		},
	}
	procSetup.DataValidation(t, payloadData)
}
