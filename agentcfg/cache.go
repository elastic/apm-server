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
	defaultExp      time.Duration = 10 * time.Second
	cleanupInterval time.Duration = 60 * time.Second
)

type cache struct {
	logger  *logp.Logger
	gocache *gocache.Cache
}

func newCache(logger *logp.Logger) *cache {
	if logger == nil {
		logger = logp.NewLogger("agentcfg")
	}
	logger.Infof("Cache creation with default expiration %v.", defaultExp)
	return &cache{
		logger:  logger,
		gocache: gocache.New(defaultExp, cleanupInterval)}
}

func (c *cache) fetchAndAdd(q Query, fn func(Query) (*Doc, error), exp time.Duration) (doc *Doc, err error) {
	id := q.id()

	// return from cache if possible
	doc, found := c.fetch(id)
	if found {
		return doc, err
	}

	// call fn to retrieve resource from external source
	doc, err = fn(q)
	if err != nil {
		return nil, err
	}

	// add resource to cache
	// use shorter expiration time for nil values
	c.add(id, doc, exp)

	return doc, err
}

func (c *cache) add(id string, doc *Doc, exp time.Duration) {
	c.gocache.Set(id, doc, exp)
	c.logger.Debugf("Cache size %v. Added ID %v.", c.gocache.ItemCount(), id)
}

func (c *cache) fetch(id string) (*Doc, bool) {
	val, found := c.gocache.Get(id)
	if !found || val == nil {
		return nil, found
	}
	return val.(*Doc), found
}
