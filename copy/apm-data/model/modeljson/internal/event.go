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

type Event struct {
	Outcome      string        `json:"outcome,omitempty"`
	Action       string        `json:"action,omitempty"`
	Dataset      string        `json:"dataset,omitempty"`
	Kind         string        `json:"kind,omitempty"`
	Category     string        `json:"category,omitempty"`
	Module       string        `json:"module,omitempty"`
	Received     Time          `json:"received,omitempty"`
	Type         string        `json:"type,omitempty"`
	SuccessCount SummaryMetric `json:"success_count,omitempty"`
	Duration     uint64        `json:"duration,omitempty"`
	Severity     uint64        `json:"severity,omitempty"`
}
