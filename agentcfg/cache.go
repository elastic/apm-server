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
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/libbeat/logp"
)

const (
	cleanupInterval time.Duration = 60 * time.Second
)

type cache struct {
	logger  *logp.Logger
	exp     time.Duration
	gocache *gocache.Cache
}

func newCache(logger *logp.Logger, exp time.Duration) *cache {
	if logger == nil {
		logger = logp.NewLogger("agentcfg")
	}
	logger.Infof("Cache creation with default expiration %v.", exp)
	return &cache{
		logger:  logger,
		exp:     exp,
		gocache: gocache.New(exp, cleanupInterval)}
}

func (c *cache) fetchAndAdd(q Query, fn func(Query) (*Doc, error)) (doc *Doc, err error) {
	id := q.ID()

	// return from cache if possible
	doc, found := c.fetch(id)
	if found {
		return
	}

	// call fn to retrieve resource from external source
	doc, err = fn(q)
	if err != nil {
		return
	}

	// add resource to cache
	c.add(id, doc)
	return
}

func (c *cache) add(id string, doc *Doc) {
	c.gocache.SetDefault(id, doc)
	if !c.logger.IsDebug() {
		return
	}
	c.logger.Debugf("Cache size %v. Added ID %v.", c.gocache.ItemCount(), id)
}

func (c *cache) fetch(id string) (*Doc, bool) {
	val, found := c.gocache.Get(id)
	if !found || val == nil {
		return nil, found
	}
	return val.(*Doc), found
}
