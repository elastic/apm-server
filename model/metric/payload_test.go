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
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/transform"
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
	ip := "127.0.0.1"
	for _, test := range []struct {
		input   map[string]interface{}
		err     error
		metrics []transform.Eventable
	}{
		{input: nil, err: nil, metrics: nil},
		{
			input:   map[string]interface{}{},
			err:     nil,
			metrics: []transform.Eventable{},
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
			metrics: []transform.Eventable{
				&metric{
					samples:   []*sample{},
					tags:      nil,
					timestamp: timestampParsed,
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
							"invalid.metric": map[string]interface{}{
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
						"tags": map[string]interface{}{
							"a.tag": "a.tag.value",
						},
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"a.counter": map[string]interface{}{
								"value": json.Number("612"),
							},
							"some.gauge": map[string]interface{}{
								"value": json.Number("9.16"),
							},
						},
					},
				},
			},
			err: nil,
			metrics: []transform.Eventable{
				&metric{
					samples: []*sample{
						{
							name:  "some.gauge",
							value: 9.16,
						},
						{
							name:  "a.counter",
							value: 612,
						},
					},
					tags: common.MapStr{
						"a.tag": "a.tag.value",
					},
					timestamp: timestampParsed,
				},
			},
		},
	} {
		transformables, err := DecodePayload(test.input)
		if test.err != nil {
			assert.Error(t, err)
		}
		// compare metrics separately as they may be ordered differently
		if test.metrics != nil {
			for i := range test.metrics {
				if test.metrics[i] == nil {
					continue
				}
				want := test.metrics[i].(*metric)
				got := transformables[i].(*metric)
				assertMetricsMatch(t, *want, *got)
			}
		}
	}
}
