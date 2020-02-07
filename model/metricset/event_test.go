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

package metricset

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/utility"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/model/metadata"
)

// assertMetricsMatch is an equality test for a metricset as sample order is not important
func assertMetricsetsMatch(t *testing.T, expected, actual Metricset) bool {
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
	spType, spSubtype, trType, trName := "db", "sql", "request", "GET /"

	for _, test := range []struct {
		input     map[string]interface{}
		err       error
		metricset *Metricset
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
			metricset: &Metricset{
				Samples:   []*Sample{},
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
			metricset: &Metricset{
				Samples: []*Sample{
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
			metricset: &Metricset{
				Samples: []*Sample{
					{
						Name:  "a.counter",
						Value: 612,
					},
				},
				Labels: common.MapStr{
					"atag": true,
				},
				Span:        &Span{Type: &spType, Subtype: &spSubtype},
				Transaction: &Transaction{Type: &trType, Name: &trName},
				Timestamp:   timestampParsed,
			},
		},
	} {
		var err error
		got, err := Decode(test.input, time.Now(), metadata.Metadata{})
		if test.err != nil {
			assert.Error(t, err)
		}

		if test.metricset != nil {
			want := test.metricset
			assertMetricsetsMatch(t, *want, *got)
		}
	}
}

func TestTransform(t *testing.T) {
	timestamp := time.Now()
	s := "myservice"
	md := metadata.NewMetadata(
		&metadata.Service{Name: &s},
		nil,
		nil,
		nil,
		nil,
	)
	spType, spSubtype, trType, trName := "db", "sql", "request", "GET /"

	tests := []struct {
		Metricset *Metricset
		Output    []common.MapStr
		Msg       string
	}{
		{
			Metricset: nil,
			Output:    nil,
			Msg:       "Nil metric",
		},
		{
			Metricset: &Metricset{Timestamp: timestamp},
			Output: []common.MapStr{
				{
					"processor": common.MapStr{"event": "metric", "name": "metric"},
					"service": common.MapStr{
						"name": "myservice",
					},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{
				Labels:    common.MapStr{"a.b": "a.b.value"},
				Timestamp: timestamp,
				Samples: []*Sample{
					{
						Name:  "a.counter",
						Value: 612,
					},
					{
						Name:  "some.gauge",
						Value: 9.16,
					},
				},
				Span:        &Span{Type: &spType, Subtype: &spSubtype},
				Transaction: &Transaction{Type: &trType, Name: &trName},
			},
			Output: []common.MapStr{
				{
					"labels": common.MapStr{
						"a.b": "a.b.value",
					},
					"service": common.MapStr{
						"name": "myservice",
					},

					"a":           common.MapStr{"counter": float64(612)},
					"some":        common.MapStr{"gauge": float64(9.16)},
					"processor":   common.MapStr{"event": "metric", "name": "metric"},
					"transaction": common.MapStr{"name": trName, "type": trType},
					"span":        common.MapStr{"type": spType, "subtype": spSubtype},
				},
			},
			Msg: "Payload with valid metric.",
		},
	}

	for idx, test := range tests {
		if test.Metricset != nil {
			test.Metricset.Metadata = *md
		}
		outputEvents := test.Metricset.Transform()

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}

func TestEventTransformUseReqTime(t *testing.T) {
	reqTimestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)

	event, error := Decode(map[string]interface{}{"samples": map[string]interface{}{}}, reqTimestampParsed, metadata.Metadata{})
	require.NoError(t, error)
	require.NotNil(t, event)
	assert.Equal(t, reqTimestampParsed, event.Timestamp)
}
