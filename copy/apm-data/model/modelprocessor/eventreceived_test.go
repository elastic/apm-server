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

package modelprocessor_test

import (
	"testing"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestRemoveEventReceived(t *testing.T) {
	processor := modelprocessor.RemoveEventReceived{}

	for _, tc := range []struct {
		name     string
		input    *modelpb.APMEvent
		expected *modelpb.APMEvent
	}{
		{
			name: "with event",
			input: &modelpb.APMEvent{
				Event: &modelpb.Event{
					Received: 1000,
				},
			},
			expected: &modelpb.APMEvent{
				Event: &modelpb.Event{
					Received: 0,
				},
			},
		},
		{
			name:     "without event",
			input:    &modelpb.APMEvent{},
			expected: &modelpb.APMEvent{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testProcessBatch(t, processor, tc.input, tc.expected)
		})
	}
}
