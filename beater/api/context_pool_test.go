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

package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/request"
)

func TestContextPool(t *testing.T) {
	// This test ensures that request.Context instances are reused from a pool, while
	// the Request stored inside a context is always set fresh
	// The test is important to avoid mixing up separate requests in a reused context.

	p := newContextPool()

	// mockhHandler adds the context and its request to dedicated slices
	var contexts, requests []interface{}
	var mu sync.Mutex
	mockHandler := func(c *request.Context) {
		mu.Lock()
		defer mu.Unlock()
		contexts = append(contexts, c)
		requests = append(requests, c.Request)
	}

	// runs 3 parallel go routines with 5 requests per go routine
	var wg sync.WaitGroup
	concRuns, runs := 10, 300
	for i := 0; i < concRuns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < runs; j++ {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				p.handler(mockHandler).ServeHTTP(w, r)
			}
		}()
	}
	wg.Wait()

	// all contexts and requests should be stored in the slices
	assert.Equal(t, runs*concRuns, len(requests))
	assert.Equal(t, runs*concRuns, len(contexts))

	// but only concRuns unique contexts should have been used,
	// while all requests must be unique.
	countUnique := func(s []interface{}) int {
		l := make(map[interface{}]struct{})
		for _, item := range s {
			if _, set := l[item]; !set {
				l[item] = struct{}{}
			}
		}
		return len(l)
	}
	totalRequests := runs * concRuns
	assert.Equal(t, totalRequests, countUnique(requests))
	assert.True(t, countUnique(contexts) < totalRequests) // contexts get reused, but not deterministic how many exactly
}
