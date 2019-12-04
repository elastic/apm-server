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

	"github.com/patrickmn/go-cache"
)

//Privileges
const (
	PrivilegeAgentConfigRead = "config_agent:read"
	PrivilegeEventWrite      = "event:write"
	PrivilegeSourcemapWrite  = "sourcemap:write"
)

var (
	//PrivilegesAll returns all available privileges
	PrivilegesAll = []string{
		PrivilegeAgentConfigRead,
		PrivilegeEventWrite,
		PrivilegeSourcemapWrite}
)

type privilegesCache struct {
	cache *cache.Cache
	size  int
}

type privileges map[string]bool

func newPrivilegesCache(expiration time.Duration, size int) *privilegesCache {
	return &privilegesCache{cache: cache.New(expiration, cleanupInterval), size: size}
}

func (c *privilegesCache) isFull() bool {
	return c.cache.ItemCount() >= c.size
}

func (c *privilegesCache) get(id string) privileges {
	if val, exists := c.cache.Get(id); exists {
		return val.(privileges)
	}
	return nil
}

func (c *privilegesCache) add(id string, privileges privileges) {
	c.cache.SetDefault(id, privileges)
}
