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

package benchtest

import (
	"sync"

	"github.com/elastic/apm-server/systemtest/loadgen/eventhandler"
)

var handlerPoolMap = make(map[string]*sync.Pool)
var handlerPoolMapMu sync.RWMutex

// NewPooledEventHandler creates or returns a previously used eventhandler.Handler.
func NewPooledEventHandler(p string, newFunc func() any) *eventhandler.Handler {
	handlerPoolMapMu.RLock()
	pool, ok := handlerPoolMap[p]
	handlerPoolMapMu.RUnlock()
	if ok {
		return pool.Get().(*eventhandler.Handler)
	}
	handlerPoolMapMu.Lock()
	pool = &sync.Pool{New: newFunc}
	handlerPoolMap[p] = pool
	handlerPoolMapMu.Unlock()
	return pool.Get().(*eventhandler.Handler)
}

// ReleaseEventHandler returns an eventhandler to its pool.
func ReleaseEventHandler(p string, eh *eventhandler.Handler) {
	handlerPoolMapMu.RLock()
	defer handlerPoolMapMu.RUnlock()
	if pool, ok := handlerPoolMap[p]; ok {
		pool.Put(eh)
	}
}
