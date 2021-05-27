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

package agentcfg

import (
	"fmt"
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/v7/libbeat/logp"
)

const (
	cleanupInterval = 60 * time.Second
)

type cache struct {
	logger  *logp.Logger
	gocache *gocache.Cache
}

func newCache(logger *logp.Logger, exp time.Duration) *cache {
	logger.Infof("Cache creation with expiration %v.", exp)
	return &cache{
		logger:  logger,
		gocache: gocache.New(exp, cleanupInterval)}
}

func (c *cache) fetch(query Query, fetch func() (Result, error)) (Result, error) {
	// return from cache if possible
	value, found := c.gocache.Get(query.id())
	if found && value != nil {
		return value.(Result), nil
	}
	// retrieve resource from external source
	result, err := fetch()
	if err != nil {
		return result, err
	}
	c.gocache.SetDefault(query.id(), result)

	if c.logger.IsDebug() {
		c.logger.Debugw(fmt.Sprintf("Cache size %v. Added ID %v.", c.gocache.ItemCount(), query.id()),
			"service.name", query.Service.Name)
	}
	return result, nil
}
