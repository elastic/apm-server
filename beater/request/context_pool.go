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

package request

import (
	"net/http"
	"sync"
)

// ContextPool provides a pool of Context objects, and a
// means of acquiring http.Handlers from Handlers.
type ContextPool struct {
	p sync.Pool
}

// NewContextPool returns a new ContextPool.
func NewContextPool() *ContextPool {
	pool := ContextPool{}
	pool.p.New = func() interface{} {
		return NewContext()
	}
	return &pool
}

// HTTPHandler returns an http.Handler that calls h with a new context.
func (pool *ContextPool) HTTPHandler(h Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := pool.p.Get().(*Context)
		defer pool.p.Put(c)
		defer c.Reset(nil, nil)
		c.Reset(w, r)
		h(c)
	})
}
