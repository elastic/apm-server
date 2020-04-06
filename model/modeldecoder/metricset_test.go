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

package modeldecoder

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/utility"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metricset"
)

// assertMetricsMatch is an equality test for a metricset as sample order is not important
func assertMetricsetsMatch(t *testing.T, expected, actual *metricset.Metricset) bool {
	samplesMatch := assert.ElementsMatch(t, expected.Samples, actual.Samples)
	expected.Samples = nil
	actual.Samples = nil
	nonSamplesMatch := assert.Equal(t, expected, actual)

	return assert.True(t, samplesMatch && nonSamplesMatch,
		fmt.Sprintf("metrics mismatch\nexpected:%#v\n   actual:%#v", expected, actual))
}

func TestDecode(t *testing.T) {
	tsFormat := func(ts time.Time) interface{} {
		return json.Number(fmt.Sprintf("%d", ts.UnixNano()/1000))
	}
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	requestTime := time.Now()
	spType, spSubtype, trType, trName := "db", "sql", "request", "GET /"
	metadata := metadata.Metadata{
		Service: metadata.Service{Name: "myservice"},
	}

	for _, test := range []struct {
		input     map[string]interface{}
		err       error
		metricset *metricset.Metricset
	}{
		{input: nil, err: nil, metricset: nil},
		{
			input:     map[string]interface{}{},
			err:       nil,
			metricset: nil,
		},
		{
			input: map[string]interface{}{
				"timestamp": tsFormat(timestampParsed),
				"samples":   map[string]interface{}{},
			},

			err: nil,
			metricset: &metricset.Metricset{
				Metadata:  metadata,
				Samples:   []*metricset.Sample{},
				Labels:    nil,
				Timestamp: timestampParsed,
			},
		},
		{
			input: map[string]interface{}{
				"timestamp": tsFormat(timestampParsed),
				"samples": map[string]interface{}{
					"invalid.metric": map[string]interface{}{
						"value": "foo",
					},
				},
			},
			err: utility.ErrFetch,
		},
		{
			input: map[string]interface{}{
				"samples": map[string]interface{}{},
			},
			metricset: &metricset.Metricset{
				Metadata:  metadata,
				Samples:   []*metricset.Sample{},
				Timestamp: requestTime,
			},
		},
		{
			input: map[string]interface{}{
				"tags": map[string]interface{}{
					"atag": true,
				},
				"timestamp": tsFormat(timestampParsed),
				"samples": map[string]interface{}{
					"a.counter": map[string]interface{}{
						"value": json.Number("612"),
					},
					"some.gauge": map[string]interface{}{
						"value": json.Number("9.16"),
					},
				},
			},
			err: nil,
			metricset: &metricset.Metricset{
				Metadata: metadata,
				Samples: []*metricset.Sample{
					{
						Name:  "some.gauge",
						Value: 9.16,
					},
					{
						Name:  "a.counter",
						Value: 612,
					},
				},
				Labels: common.MapStr{
					"atag": true,
				},
				Timestamp: timestampParsed,
			},
		},
		{
			input: map[string]interface{}{
				"tags": map[string]interface{}{
					"atag": true,
				},
				"timestamp": tsFormat(timestampParsed),
				"samples": map[string]interface{}{
					"a.counter": map[string]interface{}{
						"value": json.Number("612"),
					},
				},
				"span": map[string]interface{}{
					"type":    spType,
					"subtype": spSubtype,
				},
				"transaction": map[string]interface{}{
					"type": trType,
					"name": trName,
				},
			},
			err: nil,
			metricset: &metricset.Metricset{
				Metadata: metadata,
				Samples: []*metricset.Sample{
					{
						Name:  "a.counter",
						Value: 612,
					},
				},
				Labels: common.MapStr{
					"atag": true,
				},
				Span:        &metricset.Span{Type: &spType, Subtype: &spSubtype},
				Transaction: &metricset.Transaction{Type: &trType, Name: &trName},
				Timestamp:   timestampParsed,
			},
		},
	} {
		transformable, err := DecodeMetricset(Input{
			Raw:         test.input,
			RequestTime: requestTime,
			Metadata:    metadata,
		})
		if test.err != nil {
			assert.Error(t, err)
		}
		if test.metricset != nil {
			want := test.metricset
			assertMetricsetsMatch(t, want, transformable.(*metricset.Metricset))
		}
	}
}
