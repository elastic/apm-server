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
	// TODO(axw) emit an `event.duration` field with the duration in
	// nanoseconds, in 8.0. For now we emit event-specific duration fields.
	// See https://github.com/elastic/apm-server/issues/5999
	Duration time.Duration

	// Outcome holds the event outcome: "success", "failure", or "unknown".
	Outcome string
}

func (e *Event) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("outcome", e.Outcome)
	return common.MapStr(fields)
}
