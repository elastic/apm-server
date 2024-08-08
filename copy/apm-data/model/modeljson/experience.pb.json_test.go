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

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestUserExperienceToModelJSON(t *testing.T) {
	testCases := map[string]struct {
		proto    *modelpb.UserExperience
		expected *modeljson.UserExperience
	}{
		"empty": {
			proto:    &modelpb.UserExperience{},
			expected: &modeljson.UserExperience{},
		},
		"no pointers": {
			proto: &modelpb.UserExperience{
				CumulativeLayoutShift: 1,
				FirstInputDelay:       2,
				TotalBlockingTime:     3,
				LongTask: &modelpb.LongtaskMetrics{
					Count: 4,
					Sum:   5,
					Max:   6,
				},
			},
			expected: &modeljson.UserExperience{
				CumulativeLayoutShift: 1,
				FirstInputDelay:       2,
				TotalBlockingTime:     3,
				Longtask: modeljson.LongtaskMetrics{
					Count: 4,
					Sum:   5,
					Max:   6,
				},
			},
		},
		"full": {
			proto: &modelpb.UserExperience{
				CumulativeLayoutShift: 1,
				FirstInputDelay:       2,
				TotalBlockingTime:     3,
				LongTask: &modelpb.LongtaskMetrics{
					Count: 4,
					Sum:   5,
					Max:   6,
				},
			},
			expected: &modeljson.UserExperience{
				CumulativeLayoutShift: 1,
				FirstInputDelay:       2,
				TotalBlockingTime:     3,
				Longtask: modeljson.LongtaskMetrics{
					Count: 4,
					Sum:   5,
					Max:   6,
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var out modeljson.UserExperience
			UserExperienceModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out)
			require.Empty(t, diff)
		})
	}
}
