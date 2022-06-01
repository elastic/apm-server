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

package expvar

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test server used in the test file creates a series of natural numbers.
// Due to the queryExpvar logic to send multiple requests and aggregate them
// every term in the series is multiplied by a factor of factorBasedOnQueryExpvar
//
// TODO: @lahsivjar the logic of querying expvar leaks into these
// test cases due to the logic in test server to keep the aggregation
// deterministing for unit testing
var factorBasedOnQueryExpvar = 12

func TestStart(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	c, err := StartNewCollector(ctx, server.URL, 10*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), c.Get(Goroutines).samples)
	assert.Equal(t, int64(1)*int64(factorBasedOnQueryExpvar), c.Get(Goroutines).First)
}

func TestAggregate(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	<-time.After(55 * time.Millisecond)

	stats := c.Get(Goroutines)
	expectedMean := (float64(stats.samples*(stats.samples+1)) / float64(2*stats.samples)) * float64(factorBasedOnQueryExpvar)
	assert.GreaterOrEqual(t, stats.Last, int64(6)*int64(factorBasedOnQueryExpvar))
	assert.Equal(t, stats.Last, stats.Max)
	assert.Equal(t, stats.First, stats.Min)
	assert.Equal(t, expectedMean, stats.Mean)
}

func TestWatchMetric(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)

	watcher, err := c.WatchMetric(Goroutines, int64(5*factorBasedOnQueryExpvar))
	assert.NoError(t, err)
	select {
	case w := <-watcher:
		assert.True(t, w)
		assert.GreaterOrEqual(t, c.Get(Goroutines).Last, int64(5)*int64(factorBasedOnQueryExpvar))
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchMetricFail(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	cancel()
	<-time.After(100 * time.Millisecond)

	_, err := c.WatchMetric(Goroutines, 10)
	assert.ErrorContains(t, err, "collector has stopped")
}

func TestWatchMetricFalse(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	watcher, err := c.WatchMetric(Goroutines, 20)
	require.NoError(t, err)
	cancel()

	select {
	case w := <-watcher:
		assert.False(t, w)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchMetricNonBlocking(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	watcher, err := c.WatchMetric(Goroutines, int64(5*factorBasedOnQueryExpvar))
	require.NoError(t, err)
	// Wait for the context to be cancelled.
	<-time.After(timeout)

	// Verify the values are updated even when there isn't an active receiver
	// on the watch.
	assert.GreaterOrEqual(t, c.Get(Goroutines).Max, int64(5*factorBasedOnQueryExpvar))
	select {
	case w := <-watcher:
		assert.True(t, w)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func getTestServer(t *testing.T) *httptest.Server {
	var count uint64
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/debug/vars" {
				t.Errorf("unexpcted path: %s", r.URL.Path)
			}
			w.Header().Set(cloudProxyHeader, "1")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"beat.runtime.goroutines": %d}`, atomic.AddUint64(&count, 1))))
		}),
	)
}
