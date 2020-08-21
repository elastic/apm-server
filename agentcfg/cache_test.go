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
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"
)

var (
	defaultResult  = Result{Source{Settings: Settings{"a": "default"}, Etag: "123"}}
	externalResult = Result{Source{Settings: Settings{"a": "b"}, Etag: "123"}}
)

type cacheSetup struct {
	query  Query
	cache  *cache
	result Result
}

func newCacheSetup(service string, exp time.Duration, init bool) cacheSetup {
	setup := cacheSetup{
		query:  Query{Service: Service{Name: service}, Etag: "123"},
		cache:  newCache(logp.NewLogger(""), exp),
		result: defaultResult,
	}
	if init {
		setup.cache.gocache.SetDefault(setup.query.id(), setup.result)
	}
	return setup
}

func TestCache_fetchAndAdd(t *testing.T) {
	exp := time.Second
	for name, testCase := range map[string]struct {
		fetchFunc  func() (Result, error)
		init       bool
		doc        Result
		shouldFail bool
	}{
		"DocFromCache":         {fetchFunc: testFn, init: true, doc: defaultResult},
		"DocFromFunctionFails": {fetchFunc: testFnErr, shouldFail: true},
		"DocFromFunction":      {fetchFunc: testFn, doc: externalResult},
		"EmptyDocFromFunction": {fetchFunc: testFnSettingsNil, doc: zeroResult()},
		"NilDocFromFunction":   {fetchFunc: testFnNil},
	} {
		t.Run(name, func(t *testing.T) {
			setup := newCacheSetup(name, exp, testCase.init)

			doc, err := setup.cache.fetch(setup.query, testCase.fetchFunc)
			assert.Equal(t, testCase.doc, doc)
			if testCase.shouldFail {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
				//ensure value is cached afterwards
				cachedDoc, error := setup.cache.fetch(setup.query, testCase.fetchFunc)
				require.NoError(t, error)
				assert.Equal(t, doc, cachedDoc)
			}
		})
	}

	t.Run("CacheKeyExpires", func(t *testing.T) {
		exp := 100 * time.Millisecond
		setup := newCacheSetup(t.Name(), exp, false)
		doc, err := setup.cache.fetch(setup.query, testFn)
		require.NoError(t, err)
		require.NotNil(t, doc)
		time.Sleep(exp)
		emptyDoc, error := setup.cache.fetch(setup.query, testFnNil)
		require.NoError(t, error)
		assert.Equal(t, emptyDoc, Result{})
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
			setup.cache.fetch(setup.query, testFn)
		}
	})

	b.Run("FetchAndAddToCache", func(b *testing.B) {
		// intialize the cache, test adding random docs to cache
		// to ensure a fetch and add operation per call
		exp := 5 * time.Minute
		setup := newCacheSetup(b.Name(), exp, false)
		q := Query{Service: Service{}}
		for i := 0; i < b.N; i++ {
			q.Service.Name = fmt.Sprintf("%v", b.N)
			setup.cache.fetch(q, testFn)
		}
	})
}

func testFnErr() (Result, error) {
	return Result{}, errors.New("testFn fails")
}

func testFnNil() (Result, error) {
	return Result{}, nil
}

func testFnSettingsNil() (Result, error) {
	return zeroResult(), nil
}

func testFn() (Result, error) {
	return externalResult, nil
}
