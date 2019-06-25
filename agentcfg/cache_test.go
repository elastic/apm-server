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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"
)

var (
	defaultDoc  = Doc{Source: Source{Settings: Settings{"a": "default"}}}
	externalDoc = Doc{Source: Source{Settings: Settings{"a": "b"}}}
)

type cacheSetup struct {
	q   Query
	c   *cache
	doc *Doc
}

func newCacheSetup(service string, exp time.Duration, init bool) cacheSetup {
	setup := cacheSetup{
		q:   Query{Service: Service{Name: service}},
		c:   newCache(nil),
		doc: &defaultDoc,
	}
	if init {
		setup.c.add(setup.q.id(), setup.doc, exp)
	}
	return setup
}

func TestCache_fetchAndAdd(t *testing.T) {
	exp := time.Second
	for name, tc := range map[string]struct {
		fn   func(query Query) (*Doc, error)
		init bool

		doc  *Doc
		fail bool
	}{
		"DocFromCache":         {fn: testFn, init: true, doc: &defaultDoc},
		"DocFromFunctionFails": {fn: testFnErr, fail: true},
		"DocFromFunction":      {fn: testFn, doc: &externalDoc},
		"EmptyDocFromFunction": {fn: testFnSettingsNil, doc: &Doc{Source: Source{Settings: Settings{}}}},
		"NilDocFromFunction":   {fn: testFnNil},
	} {
		t.Run(name, func(t *testing.T) {
			setup := newCacheSetup(name, exp, tc.init)

			doc, err := setup.c.fetchAndAdd(setup.q, tc.fn, exp)
			assert.Equal(t, tc.doc, doc)
			if tc.fail {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
				//ensure value is cached afterwards
				cachedDoc, found := setup.c.fetch(setup.q.id())
				assert.True(t, found)
				assert.Equal(t, doc, cachedDoc)
			}
		})
	}

	t.Run("CacheKeyExpires", func(t *testing.T) {
		exp := 100 * time.Millisecond
		setup := newCacheSetup(t.Name(), exp, false)
		doc, err := setup.c.fetchAndAdd(setup.q, testFn, exp)
		require.NoError(t, err)
		require.NotNil(t, doc)
		time.Sleep(exp)
		nilDoc, found := setup.c.fetch(setup.q.id())
		assert.False(t, found)
		assert.Nil(t, nilDoc)
	})
}

func BenchmarkFetchAndAdd(b *testing.B) {
	// this micro benchmark only accounts for the underlying cache
	// providing some benchmark baseline in case the cache library changes in the future
	// It does not compare cache vs. external call, as this should rather be embedded in
	// some integration test. It also doesn't account for potentially increased CPU usage through
	// background processes taking care of expiring keys.

	b.Run("FetchFromCache", func(b *testing.B) {
		// intialize the cache and add a document to it before the benchmark,
		// to ensure docs are only fetched from cache
		exp := 5 * time.Minute
		setup := newCacheSetup(b.Name(), exp, true)
		for i := 0; i < b.N; i++ {
			setup.c.fetchAndAdd(setup.q, testFn, exp)
		}
	})

	b.Run("FetchAndAddToCache", func(b *testing.B) {
		// intialize the cache, test adding random docs to cache
		// to ensure a fetch and add operation per call
		exp := 5 * time.Minute
		setup := newCacheSetup(b.Name(), exp, false)
		q := Query{Service: Service{}}
		for i := 0; i < b.N; i++ {
			q.Service.Name = string(b.N)
			setup.c.fetchAndAdd(q, testFn, exp)
		}
	})
}

func testFnErr(_ Query) (*Doc, error) {
	return nil, errors.New("testFn fails")
}

func testFnNil(_ Query) (*Doc, error) {
	return nil, nil
}

func testFnSettingsNil(_ Query) (*Doc, error) {
	return &Doc{Source: Source{Settings: Settings{}}}, nil
}

func testFn(_ Query) (*Doc, error) {
	return &externalDoc, nil
}
