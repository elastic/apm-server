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

	"github.com/elastic/elastic-agent-libs/mapstr"
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

	// OutcomeNumeric TODO
	OutcomeNumeric *SummaryMetric

	// Severity holds the numeric severity of the event for log events.
	Severity int64

	// Action holds the action captured by the event for log events.
	Action string

	// Dataset holds the the dataset which produces the events. If an event
	// source publishes more than one type of log or events (e.g. access log,
	// error log), the dataset is used to specify which one the event comes from.
	Dataset string
}

func (e *Event) fields() mapstr.M {
	var fields mapStr
	fields.maybeSetString("outcome", e.Outcome)
	if e.OutcomeNumeric != nil {
		fields.maybeSetMapStr("outcome_numeric", e.OutcomeNumeric.fields())
	}
	fields.maybeSetString("action", e.Action)
	fields.maybeSetString("dataset", e.Dataset)
	if e.Severity > 0 {
		fields.set("severity", e.Severity)
	}
	if e.Duration > 0 {
		fields.set("duration", e.Duration.Nanoseconds())
	}
	return mapstr.M(fields)
}
