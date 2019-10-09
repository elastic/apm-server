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

type m map[string]interface{}

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

func policyPool() policies {
	return policies{
		rollover7Days: {
			policyStr: m{
				phasesStr: m{
					hotStr: m{
						actionsStr: m{
							rolloverStr: m{
								maxSizeStr: "50gb",
								maxAgeStr:  "7d",
							},
							setPriorityStr: m{
								priorityStr: 100,
							},
						},
					},
					warmStr: m{
						minAgeStr: "31d",
						actionsStr: m{
							setPriorityStr: m{
								priorityStr: 50,
							},
							readonlyStr: m{},
						},
					},
				},
			},
		},
		rollover1Day: {
			policyStr: m{
				phasesStr: m{
					hotStr: m{
						actionsStr: m{
							rolloverStr: m{
								maxSizeStr: "50gb",
								maxAgeStr:  "1d",
							},
							setPriorityStr: m{
								priorityStr: 100,
							},
						},
					},
					warmStr: m{
						minAgeStr: "7d",
						actionsStr: m{
							setPriorityStr: m{
								priorityStr: 50,
							},
							readonlyStr: m{},
						},
					},
				},
			},
		},
	}
}
