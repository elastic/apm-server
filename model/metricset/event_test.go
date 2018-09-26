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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elastic/apm-server/model/metadata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

// assertMetricsMatch is an equality test for a metric as sample order is not important
func assertMetricsMatch(t *testing.T, expected, actual Metric) bool {
	samplesMatch := assert.ElementsMatch(t, expected.Samples, actual.Samples)
	expected.Samples = nil
	actual.Samples = nil
	nonSamplesMatch := assert.Equal(t, expected, actual)

	return assert.True(t, samplesMatch && nonSamplesMatch,
		fmt.Sprintf("metrics mismatch\nexpected:%#v\n   actual:%#v", expected, actual))
}

func TestDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	// ip := "127.0.0.1"
	for _, test := range []struct {
		input  map[string]interface{}
		err    error
		metric *Metric
	}{
		{input: nil, err: nil, metric: nil},
		{
			input:  map[string]interface{}{},
			err:    nil,
			metric: nil,
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"samples":   map[string]interface{}{},
			},

			err: nil,
			metric: &Metric{
				Samples:   []*Sample{},
				Tags:      nil,
				Timestamp: timestampParsed,
			},
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"samples": map[string]interface{}{
					"invalid.metric": map[string]interface{}{
						"value": "foo",
					},
				},
			},
			err: errors.New("Error fetching field"),
		},
		{
			input: map[string]interface{}{
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
			err: nil,
			metric: &Metric{
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
				Tags: common.MapStr{
					"a.tag": "a.tag.value",
				},
				Timestamp: timestampParsed,
			},
		},
	} {
		var err error
		transformables, err := DecodeEvent(test.input, err)
		if test.err != nil {
			assert.Error(t, err)
		}

		if test.metric != nil {
			want := test.metric
			got := transformables.(*Metric)
			assertMetricsMatch(t, *want, *got)
		}
	}
}

func TestTransform(t *testing.T) {
	timestamp := time.Now()
	md := metadata.NewMetadata(
		&metadata.Service{Name: "myservice"},
		nil,
		nil,
		nil,
	)

	tests := []struct {
		Metric *Metric
		Output []common.MapStr
		Msg    string
	}{
		{
			Metric: nil,
			Output: nil,
			Msg:    "Nil metric",
		},
		{
			Metric: &Metric{Timestamp: timestamp},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myservice",
						},
					},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Metric: &Metric{
				Tags:      common.MapStr{"a.tag": "a.tag.value"},
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
					"a":         common.MapStr{"counter": float64(612)},
					"some":      common.MapStr{"gauge": float64(9.16)},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with valid metric.",
		},
	}

	tctx := &transform.Context{Config: transform.Config{}, Metadata: *md}
	for idx, test := range tests {
		outputEvents := test.Metric.Transform(tctx)

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}

func TestEventTransformUseReqTime(t *testing.T) {
	reqTimestamp := "2017-05-30T18:53:27.154Z"
	reqTimestampParsed, err := time.Parse(time.RFC3339, reqTimestamp)
	require.NoError(t, err)

	e := Metric{}
	beatEvent := e.Transform(&transform.Context{RequestTime: reqTimestampParsed})
	require.Len(t, beatEvent, 1)
	assert.Equal(t, reqTimestampParsed, beatEvent[0].Timestamp)
}
