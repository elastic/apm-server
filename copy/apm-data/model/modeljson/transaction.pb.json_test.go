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

package modeljson

import (
	"testing"
	"time"

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestTransactionToModelJSON(t *testing.T) {
	testCases := map[string]struct {
		proto               *modelpb.Transaction
		expectedNoMetricset *modeljson.Transaction
		expectedMetricset   *modeljson.Transaction
	}{
		"empty": {
			proto:               &modelpb.Transaction{},
			expectedNoMetricset: &modeljson.Transaction{},
			expectedMetricset:   &modeljson.Transaction{},
		},
		"no pointers": {
			proto: &modelpb.Transaction{
				Type:                "type",
				Name:                "name",
				Result:              "result",
				Id:                  "id",
				RepresentativeCount: 8,
				Sampled:             true,
				Root:                true,
			},
			expectedNoMetricset: &modeljson.Transaction{
				Type:                "type",
				Name:                "name",
				Result:              "result",
				ID:                  "id",
				RepresentativeCount: 8,
				Sampled:             true,
				Root:                true,
			},
			expectedMetricset: &modeljson.Transaction{
				Type:                "type",
				Name:                "name",
				Result:              "result",
				ID:                  "id",
				RepresentativeCount: 8,
				Sampled:             true,
				Root:                true,
			},
		},
		"full": {
			proto: &modelpb.Transaction{
				SpanCount: &modelpb.SpanCount{
					Started: uintPtr(1),
					Dropped: uintPtr(2),
				},
				// TODO investigat valid values
				Custom: nil,
				Marks: map[string]*modelpb.TransactionMark{
					"foo": {
						Measurements: map[string]float64{
							"bar": 3,
						},
					},
				},
				Type:   "type",
				Name:   "name",
				Result: "result",
				Id:     "id",
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{4},
					Counts: []uint64{5},
				},
				DroppedSpansStats: []*modelpb.DroppedSpanStats{
					{
						DestinationServiceResource: "destinationserviceresource",
						ServiceTargetType:          "servicetargetype",
						ServiceTargetName:          "servicetargetname",
						Outcome:                    "outcome",
						Duration: &modelpb.AggregatedDuration{
							Count: 4,
							Sum:   uint64(5 * time.Second),
						},
					},
				},
				DurationSummary: &modelpb.SummaryMetric{
					Count: 6,
					Sum:   7,
				},
				RepresentativeCount:   8,
				Sampled:               true,
				Root:                  true,
				ProfilerStackTraceIds: []string{"foo", "foo", "bar"},
			},
			expectedNoMetricset: &modeljson.Transaction{
				SpanCount: modeljson.SpanCount{
					Started: uintPtr(1),
					Dropped: uintPtr(2),
				},
				// TODO investigat valid values
				Custom: nil,
				Marks: map[string]map[string]float64{
					"foo": {
						"bar": 3,
					},
				},
				Type:   "type",
				Name:   "name",
				Result: "result",
				ID:     "id",
				DurationHistogram: modeljson.Histogram{
					Values: []float64{4},
					Counts: []uint64{5},
				},
				DurationSummary: modeljson.SummaryMetric{
					Count: 6,
					Sum:   7,
				},
				RepresentativeCount:   8,
				Sampled:               true,
				Root:                  true,
				ProfilerStackTraceIds: []string{"foo", "foo", "bar"},
			},
			expectedMetricset: &modeljson.Transaction{
				SpanCount: modeljson.SpanCount{
					Started: uintPtr(1),
					Dropped: uintPtr(2),
				},
				// TODO investigat valid values
				Custom: nil,
				Marks: map[string]map[string]float64{
					"foo": {
						"bar": 3,
					},
				},
				Type:   "type",
				Name:   "name",
				Result: "result",
				ID:     "id",
				DurationHistogram: modeljson.Histogram{
					Values: []float64{4},
					Counts: []uint64{5},
				},
				DroppedSpansStats: []modeljson.DroppedSpanStats{
					{
						DestinationServiceResource: "destinationserviceresource",
						ServiceTargetType:          "servicetargetype",
						ServiceTargetName:          "servicetargetname",
						Outcome:                    "outcome",
						Duration: modeljson.AggregatedDuration{
							Count: 4,
							Sum:   5 * time.Second,
						},
					},
				},
				DurationSummary: modeljson.SummaryMetric{
					Count: 6,
					Sum:   7,
				},
				RepresentativeCount:   8,
				Sampled:               true,
				Root:                  true,
				ProfilerStackTraceIds: []string{"foo", "foo", "bar"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var out modeljson.Transaction
			TransactionModelJSON(tc.proto, &out, false)
			require.Empty(t, cmp.Diff(*tc.expectedNoMetricset, out))

			var out2 modeljson.Transaction
			TransactionModelJSON(tc.proto, &out2, true)
			require.Empty(t, cmp.Diff(*tc.expectedMetricset, out2))
		})
	}
}
