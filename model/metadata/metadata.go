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

package metadata

import (
	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

type Metadata struct {
	Service *Service
	Process *Process
	System  *System
	User    *User
	Labels  common.MapStr
}

func (m *Metadata) Set(fields common.MapStr) common.MapStr {
	containerFields := m.System.containerFields()
	hostFields := m.System.fields()
	utility.Set(fields, "service", m.Service.Fields(get(containerFields, "id"), get(hostFields, "name")))
	utility.Set(fields, "agent", m.Service.AgentFields())
	utility.Set(fields, "host", hostFields)
	utility.Set(fields, "process", m.Process.fields())
	utility.Set(fields, "user", m.User.Fields())
	utility.Set(fields, "client", m.User.ClientFields())
	utility.Set(fields, "user_agent", m.User.UserAgentFields())
	utility.Set(fields, "container", containerFields)
	utility.Set(fields, "kubernetes", m.System.kubernetesFields())
	// to be merged with specific event labels, these should be overwritten in case of conflict
	utility.Set(fields, "labels", m.Labels)
	return fields
}

func get(m common.MapStr, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
