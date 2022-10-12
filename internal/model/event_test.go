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
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/stretchr/testify/assert"
)

func TestEventFieldsEmpty(t *testing.T) {
	event := APMEvent{}
	beatEvent := event.BeatEvent()
	assert.Empty(t, beatEvent.Fields)
}

func TestEventFields(t *testing.T) {
	tests := map[string]struct {
		Event  Event
		Output mapstr.M
	}{
		"withOutcome": {
			Event: Event{Outcome: "success"},
			Output: mapstr.M{
				"outcome": "success",
			},
		},
		"withAction": {
			Event: Event{Action: "process-started"},
			Output: mapstr.M{
				"action": "process-started",
			},
		},
		"withDataset": {
			Event: Event{Dataset: "access-log"},
			Output: mapstr.M{
				"dataset": "access-log",
			},
		},
		"withSeverity": {
			Event: Event{Severity: 1},
			Output: mapstr.M{
				"severity": int64(1),
			},
		},
		"withDuration": {
			Event: Event{Duration: time.Minute},
			Output: mapstr.M{
				"duration": int64(60000000000),
			},
		},
		"withOutcomeActionDataset": {
			Event: Event{Outcome: "success", Action: "process-started", Dataset: "access-log"},
			Output: mapstr.M{
				"outcome": "success",
				"action":  "process-started",
				"dataset": "access-log",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event := APMEvent{Event: tc.Event}
			beatEvent := event.BeatEvent()
			assert.Equal(t, tc.Output, beatEvent.Fields["event"])
		})
	}
}
