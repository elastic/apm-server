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

import "github.com/elastic/beats/libbeat/common"

type m common.MapStr

var eventPolicies = map[string]common.MapStr{
	"error":       PolicyKeepMedium,
	"span":        PolicyKeepMedium,
	"transaction": PolicyKeep,
	"metric":      PolicyKeep,
}

//PolicyKeep should be used for indices queried max for 2 months
var PolicyKeep = common.MapStr{
	"policy": m{
		"phases": m{
			"hot": m{
				"actions": m{
					"rollover": m{
						"max_size": "50gb",
						"max_age":  "7d",
					},
					"set_priority": m{
						"priority": 100,
					},
				},
			},
			"warm": m{
				"min_age": "31d",
				"actions": m{
					"set_priority": m{
						"priority": 50,
					},
					"readonly": m{},
				},
			},
		},
	},
}

//PolicyKeepMedium should be used for indices that need to be queried max for two weeks
var PolicyKeepMedium = common.MapStr{
	"policy": m{
		"phases": m{
			"hot": m{
				"actions": m{
					"rollover": m{
						"max_size": "50gb",
						"max_age":  "1d",
					},
					"set_priority": m{
						"priority": 100,
					},
				},
			},
			"warm": m{
				"min_age": "7d",
				"actions": m{
					"set_priority": m{
						"priority": 50,
					},
					"readonly": m{},
				},
			},
		},
	},
}
