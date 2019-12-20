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

package authorization

import (
	"time"

	es "github.com/elastic/apm-server/elasticsearch"

	"github.com/patrickmn/go-cache"
)

var (
	PrivilegeAgentConfigRead = es.NewPrivilege("agentConfig", "config_agent:read")
	PrivilegeEventWrite      = es.NewPrivilege("event", "event:write")
	PrivilegeSourcemapWrite  = es.NewPrivilege("sourcemap", "sourcemap:write")
	PrivilegesAll            = []es.NamedPrivilege{PrivilegeAgentConfigRead, PrivilegeEventWrite, PrivilegeSourcemapWrite}
	// ActionAny can't be used for querying, use ActionsAll instead
	ActionAny  = es.PrivilegeAction("*")
	ActionsAll = func() []es.PrivilegeAction {
		actions := make([]es.PrivilegeAction, 0)
		for _, privilege := range PrivilegesAll {
			actions = append(actions, privilege.Action)
		}
		return actions
	}
)

type privilegesCache struct {
	cache *cache.Cache
	size  int
}

func newPrivilegesCache(expiration time.Duration, size int) *privilegesCache {
	return &privilegesCache{cache: cache.New(expiration, cleanupInterval), size: size}
}

func (c *privilegesCache) isFull() bool {
	return c.cache.ItemCount() >= c.size
}

func (c *privilegesCache) get(id string) es.Permissions {
	if val, exists := c.cache.Get(id); exists {
		return val.(es.Permissions)
	}
	return nil
}

func (c *privilegesCache) add(id string, privileges es.Permissions) {
	c.cache.SetDefault(id, privileges)
}
