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
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
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

func newCacheSetup(t testing.TB, service string, exp time.Duration, init bool) cacheSetup {
	cache, err := newCache(logptest.NewTestingLogger(t, ""), exp)
	require.NoError(t, err)
	setup := cacheSetup{
		query:  Query{Service: Service{Name: service}, Etag: "123"},
		cache:  cache,
		result: defaultResult,
	}
	if init {
		setup.cache.gocache.Add(setup.query.id(), setup.result)
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
			setup := newCacheSetup(t, name, exp, testCase.init)

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
		setup := newCacheSetup(t, t.Name(), exp, false)
		doc, err := setup.cache.fetch(setup.query, testFn)
		require.NoError(t, err)
		require.NotNil(t, doc)
		time.Sleep(exp)
		emptyDoc, error := setup.cache.fetch(setup.query, testFnNil)
		require.NoError(t, error)
		assert.Equal(t, emptyDoc, Result{})
	})
}

func TestCache_Collisions(t *testing.T) {
	const (
		svcA  = "a"
		svcAB = "ab"

		envBC = "bc"
		envC  = "c"
	)

	var (
		serviceAResult  = Result{Source: Source{Agent: "svc_a"}}
		serviceABResult = Result{Source: Source{Agent: "svc_ab"}}
	)

	// initialize empty cache
	cacheTTL := time.Minute
	testCache, err := newCache(logptest.NewTestingLogger(t, ""), cacheTTL)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		query    Query
		expected Result
	}{
		{
			name:     "svc ab",
			query:    Query{Service: Service{Name: svcAB, Environment: envC}},
			expected: serviceABResult,
		},
		{
			name:     "svc a",
			query:    Query{Service: Service{Name: svcA, Environment: envBC}},
			expected: serviceAResult,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// create mock external source
			mockFetcher := func() (Result, error) {
				switch {
				case tc.query.Service.Name == svcAB && tc.query.Service.Environment == envC:
					return serviceABResult, nil
				case tc.query.Service.Name == svcA && tc.query.Service.Environment == envBC:
					return serviceAResult, nil
				}
				return Result{Source: Source{Agent: "other_agent"}}, nil
			}

			got, err := testCache.fetch(tc.query, mockFetcher)
			require.NoError(t, err)

			if tc.expected.Source.Agent != got.Source.Agent {
				t.Errorf("Expected %v, instead found %v", tc.expected.Source.Agent, got.Source.Agent)
			}
		})
	}
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
		setup := newCacheSetup(b, b.Name(), exp, true)
		for i := 0; i < b.N; i++ {
			setup.cache.fetch(setup.query, testFn)
		}
	})

	b.Run("FetchAndAddToCache", func(b *testing.B) {
		// intialize the cache, test adding random docs to cache
		// to ensure a fetch and add operation per call
		exp := 5 * time.Minute
		setup := newCacheSetup(b, b.Name(), exp, false)
		setup.cache.logger = logp.NewNopLogger()
		q := Query{Service: Service{}}
		for i := 0; i < b.N; i++ {
			q.Service.Name = fmt.Sprintf("%v", b.N)
			setup.cache.fetch(q, testFn)
		}
	})
}

func BenchmarkAddToCache(b *testing.B) {
	// create initial list of queries
	const cacheSize = 8000
	queries := make([]Query, cacheSize)
	for i := range cacheSize {
		queries[i] = Query{
			Service: Service{
				Environment: "production",
			},
		}
	}

	// create a cache once
	cache, err := newCache(logp.NewNopLogger(), time.Minute)
	require.NoError(b, err)

	var adds int64
	var nextQueryID int
	for b.Loop() {
		// update queries with a new service name on each loop iteration
		b.StopTimer()
		for i := range queries {
			queries[i].Service.Name = strconv.Itoa(nextQueryID)
			nextQueryID++
		}
		b.StartTimer()

		// insert queries
		for _, query := range queries {
			cache.gocache.Add(query.id(), externalResult)
		}
		adds += int64(len(queries))
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(adds), "ns/add")
	b.ReportMetric(0, "ns/op")
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
