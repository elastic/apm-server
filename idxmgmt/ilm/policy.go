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

package ilm

const (
	rollover1Day  = "rollover-1-day"
	rollover7Days = "rollover-7-days"

	policyStr      = "policy"
	phasesStr      = "phases"
	hotStr         = "hot"
	warmStr        = "warm"
	actionsStr     = "actions"
	rolloverStr    = "rollover"
	maxSizeStr     = "max_size"
	maxAgeStr      = "max_age"
	minAgeStr      = "min_age"
	setPriorityStr = "set_priority"
	priorityStr    = "priority"
	readonlyStr    = "readonly"

	errorEvent       = "error"
	spanEvent        = "span"
	transactionEvent = "transaction"
	metricEvent      = "metric"
)

func policyMapping() map[string]string {
	return map[string]string{
		errorEvent:       rollover1Day,
		spanEvent:        rollover1Day,
		transactionEvent: rollover7Days,
		metricEvent:      rollover7Days,
	}
}

func policyPool() map[string]policyBody {
	return map[string]policyBody{
		rollover7Days: {
			policyStr: map[string]interface{}{
				phasesStr: map[string]interface{}{
					hotStr: map[string]interface{}{
						actionsStr: map[string]interface{}{
							rolloverStr: map[string]interface{}{
								maxSizeStr: "50gb",
								maxAgeStr:  "7d",
							},
							setPriorityStr: map[string]interface{}{
								priorityStr: 100,
							},
						},
					},
					warmStr: map[string]interface{}{
						minAgeStr: "31d",
						actionsStr: map[string]interface{}{
							setPriorityStr: map[string]interface{}{
								priorityStr: 50,
							},
							readonlyStr: map[string]interface{}{},
						},
					},
				},
			},
		},
		rollover1Day: {
			policyStr: map[string]interface{}{
				phasesStr: map[string]interface{}{
					hotStr: map[string]interface{}{
						actionsStr: map[string]interface{}{
							rolloverStr: map[string]interface{}{
								maxSizeStr: "50gb",
								maxAgeStr:  "1d",
							},
							setPriorityStr: map[string]interface{}{
								priorityStr: 100,
							},
						},
					},
					warmStr: map[string]interface{}{
						minAgeStr: "7d",
						actionsStr: map[string]interface{}{
							setPriorityStr: map[string]interface{}{
								priorityStr: 50,
							},
							readonlyStr: map[string]interface{}{},
						},
					},
				},
			},
		},
	}
}
