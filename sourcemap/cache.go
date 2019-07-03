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

package sourcemap

import (
	"math"
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const (
	minCleanupIntervalSeconds float64 = 60
)

type cache struct {
	goca   *gocache.Cache
	logger *logp.Logger
}

func newCache(expiration time.Duration) (*cache, error) {
	if expiration < 0 {
		return nil, Error{
			Msg:  "Cache cannot be initialized. Expiration and CleanupInterval need to be >= 0",
			Kind: InitError,
		}
	}
	return &cache{
		goca:   gocache.New(expiration, cleanupInterval(expiration)),
		logger: logp.NewLogger(logs.Sourcemap),
	}, nil
}

func (c *cache) add(id Id, consumer *sourcemap.Consumer) {
	c.goca.Set(id.Key(), consumer, gocache.DefaultExpiration)
	if !c.logger.IsDebug() {
		return
	}
	c.logger.Debugf("Added id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
}

func (c *cache) remove(id Id) {
	c.goca.Delete(id.Key())
	if !c.logger.IsDebug() {
		return
	}
	c.logger.Debugf("Removed id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
}

func (c *cache) fetch(id Id) (*sourcemap.Consumer, bool) {
	if cached, found := c.goca.Get(id.Key()); found {
		if cached == nil {
			// in case empty value was cached
			// return found=true
			return nil, true
		}
		return cached.(*sourcemap.Consumer), true
	}
	return nil, false
}

func cleanupInterval(ttl time.Duration) time.Duration {
	return time.Duration(math.Max(ttl.Seconds(), minCleanupIntervalSeconds)) * time.Second
}
