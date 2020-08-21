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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestUserExperienceFields(t *testing.T) {
	tests := []struct {
		Input    *UserExperience
		Expected common.MapStr
	}{{
		Input:    nil,
		Expected: nil,
	}, {
		Input: &UserExperience{
			CumulativeLayoutShift: -1,
			FirstInputDelay:       -1,
			TotalBlockingTime:     -1,
		},
		Expected: nil,
	}, {
		Input: &UserExperience{
			CumulativeLayoutShift: 1,
			FirstInputDelay:       2.3,
			TotalBlockingTime:     4.56,
		},
		Expected: common.MapStr{
			"cls": 1.0,
			"fid": 2.3,
			"tbt": 4.56,
		},
	}}
	for _, test := range tests {
		output := test.Input.Fields()
		assert.Equal(t, test.Expected, output)
	}
}
