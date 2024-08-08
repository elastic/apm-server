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

const (
	testTime = "2006-01-02T15:04:05Z"
)

func TestEventToModelJSON(t *testing.T) {
	now, err := time.Parse(time.RFC3339, testTime)
	require.NoError(t, err)

	testCases := map[string]struct {
		proto    *modelpb.Event
		expected *modeljson.Event
	}{
		"empty": {
			proto:    &modelpb.Event{},
			expected: &modeljson.Event{},
		},
		"full": {
			proto: &modelpb.Event{
				Outcome:  "outcome",
				Action:   "action",
				Dataset:  "dataset",
				Kind:     "kind",
				Category: "category",
				Type:     "type",
				SuccessCount: &modelpb.SummaryMetric{
					Count: 1,
					Sum:   2,
				},
				Duration: uint64(3 * time.Second),
				Severity: 4,
				Received: modelpb.FromTime(now),
			},
			expected: &modeljson.Event{
				Outcome:  "outcome",
				Action:   "action",
				Dataset:  "dataset",
				Kind:     "kind",
				Category: "category",
				Type:     "type",
				SuccessCount: modeljson.SummaryMetric{
					Count: 1,
					Sum:   2,
				},
				Duration: uint64(3 * time.Second),
				Severity: 4,
				Received: modeljson.Time(now),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var out modeljson.Event
			EventModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out,
				cmp.Comparer(func(a modeljson.Time, b modeljson.Time) bool {
					return time.Time(a).Equal(time.Time(b))
				}))
			require.Empty(t, diff)
		})
	}
}
