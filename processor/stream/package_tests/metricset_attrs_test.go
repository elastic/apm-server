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

package package_tests

import (
	"encoding/json"
	"testing"

	"github.com/elastic/apm-server/model/metricset/generated/schema"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func metricsetProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc:            &V2TestProcessor{StreamProcessor: stream.StreamProcessor{}},
		FullPayloadPath: "../testdata/intake-v2/metricsets.ndjson",
		TemplatePaths: []string{
			"../../../model/metricset/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
		Schema: schema.ModelSchema,
	}
}

func TestAttributesPresenceInMetric(t *testing.T) {
	requiredKeys := tests.NewSet(
		"service",
		"metricset",
		"metricset.samples",
		"metricset.timestamp",
		"metricset.samples.+.value",
	)
	metricsetProcSetup().AttrsPresence(t, requiredKeys, nil)
}

func TestInvalidPayloads(t *testing.T) {
	type obj = map[string]interface{}
	type val = []interface{}

	validMetric := obj{"value": json.Number("1.0")}
	// every payload needs a timestamp, these reduce the clutter
	//tsk, ts := "timestamp", "2017-05-30T18:53:42.281Z"

	payloadData := []tests.SchemaTestData{
		{Key: "metricset.timestamp",
			Valid: val{json.Number("1496170422281000")},
			Invalid: []tests.Invalid{
				{Msg: `timestamp/type`, Values: val{"1496170422281000"}}}},
		{Key: "metricset.tags",
			Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
			Invalid: []tests.Invalid{
				{Msg: `tags/type`, Values: val{"tags"}},
				{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
				{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}}}},
		},
		{
			Key: "metricset.samples",
			Valid: val{
				obj{"valid-metric": validMetric},
			},
			Invalid: []tests.Invalid{
				{
					Msg: "/properties/samples/additionalproperties",
					Values: val{
						obj{"metric\"key\"_quotes": validMetric},
						obj{"metric-*-key-star": validMetric},
					},
				},
				{
					Msg: "/properties/samples/patternproperties",
					Values: val{
						obj{"nil-value": obj{"value": nil}},
						obj{"string-value": obj{"value": "foo"}},
					},
				},
			},
		},
	}
	metricsetProcSetup().DataValidation(t, payloadData)
}
