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
	"time"

	"github.com/elastic/beats/v7/libbeat/common"
)

// Event holds information about an event, in ECS terms.
//
// https://www.elastic.co/guide/en/ecs/current/ecs-event.html
type Event struct {
	// Duration holds the event duration.
	//
	// Duration is only added as a field (`duration`) if greater than zero.
	Duration time.Duration

	// Outcome holds the event outcome: "success", "failure", or "unknown".
	Outcome string

	// Severity holds the numeric severity of the event for log events.
	Severity int64

	// Severity holds the action captured by the event for log events.
	Action string
}

func (e *Event) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("outcome", e.Outcome)
	fields.maybeSetString("action", e.Action)
	if e.Severity > 0 {
		fields.set("severity", e.Severity)
	}
	if e.Duration > 0 {
		fields.set("duration", e.Duration.Nanoseconds())
	}
	return common.MapStr(fields)
}
