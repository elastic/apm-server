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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	serverURL := setupTestServer(t, 1, make(chan bool, 1))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := StartNewCollector(ctx, serverURL, 10*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), c.Get(Goroutines).samples)
	assert.Equal(t, int64(1), c.Get(Goroutines).First)
	assert.Equal(t, int64(1), c.Get(Goroutines).Last)
}

func TestAggregate(t *testing.T) {
	done := make(chan bool)
	samples := 5
	serverURL := setupTestServer(t, samples, done)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := StartNewCollector(ctx, serverURL, 10*time.Millisecond)
	assert.NoError(t, err)
	assert.True(t, <-done)
	stats := c.Get(Goroutines)
	// asserting with samples-1 since done on samples means atleast samples-1 events are fully consumed
	assert.GreaterOrEqual(t, stats.Last, int64(samples-1))
	expectedMean := (float64(stats.samples*(stats.samples+1)) / float64(2*stats.samples))
	assert.Equal(t, expectedMean, stats.Mean)
	assert.Equal(t, stats.Last, stats.Max)
	assert.Equal(t, stats.First, stats.Min)
}

func TestWatchMetric(t *testing.T) {
	serverURL := setupTestServer(t, 5, make(chan bool, 1))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := StartNewCollector(ctx, serverURL, 10*time.Millisecond)
	assert.NoError(t, err)
	watcher, err := c.WatchMetric(Goroutines, int64(5))
	assert.NoError(t, err)
	select {
	case w := <-watcher:
		assert.True(t, w)
		assert.Equal(t, c.Get(Goroutines).Last, int64(5))
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchMetricFail(t *testing.T) {
	serverURL := setupTestServer(t, 1, make(chan bool, 1))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c, err := StartNewCollector(ctx, serverURL, 10*time.Millisecond)
	assert.NoError(t, err)
	cancel()
	<-time.After(100 * time.Millisecond)
	_, err = c.WatchMetric(Goroutines, 10)
	assert.ErrorContains(t, err, "collector has stopped")
}

func TestWatchMetricFalse(t *testing.T) {
	serverURL := setupTestServer(t, 1, make(chan bool, 1))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c, err := StartNewCollector(ctx, serverURL, 10*time.Millisecond)
	assert.NoError(t, err)
	watcher, err := c.WatchMetric(Goroutines, 20)
	assert.NoError(t, err)
	cancel()

	select {
	case w := <-watcher:
		assert.False(t, w)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchMetricNonBlocking(t *testing.T) {
	done := make(chan bool)
	serverURL := setupTestServer(t, 5, done)
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := StartNewCollector(ctx, serverURL, 10*time.Millisecond)
	assert.NoError(t, err)
	watcher, err := c.WatchMetric(Goroutines, int64(5))
	assert.NoError(t, err)
	// Wait for the context to be cancelled.
	<-time.After(timeout)
	assert.True(t, <-done)

	// Verify the values are updated even when there isn't an active receiver
	// on the watch.
	assert.GreaterOrEqual(t, c.Get(Goroutines).Max, int64(5))
	select {
	case w := <-watcher:
		assert.True(t, w)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

// setupTestServer creates a test http server with following properties:
//   - max number of responses are controlled using channels
//   - max number of responses is configured using count which represents
//     the number of samples that the collector witnesses
//   - if the responses are not consumed in 1 second the server returns error
//   - each metric creates a timeseries of natural numbers
//
// TODO: @lahsivjar the logic of querying expvar multiple times leaks into
// these test cases due to the logic in test server to keep the aggregation
// deterministic for unit testing
func setupTestServer(t *testing.T, count int, done chan<- bool) string {
	factorBasedOnQueryExpvar := 12
	resChan := make(chan string)
	timeout := time.Second
	server := getTestServer(t, resChan, timeout)
	t.Cleanup(func() {
		server.Close()
	})

	go func() {
		var val int
		for i := 0; i < count; i++ {
			val++
			for j := 0; j < factorBasedOnQueryExpvar; j++ {
				select {
				case resChan <- fmt.Sprintf(`{"beat.runtime.goroutines": %d}`, val):
				case <-time.After(timeout):
					done <- false
					return
				}
			}
		}
		done <- true
	}()

	return server.URL
}

func getTestServer(t *testing.T, resChan <-chan string, timeout time.Duration) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/debug/vars" {
				t.Errorf("unexpcted path: %s", r.URL.Path)
			}
			select {
			case res := <-resChan:
				w.Header().Set(cloudProxyHeader, "1")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(res))
			case <-time.After(timeout):
				w.WriteHeader(http.StatusInternalServerError)
			}
		}),
	)
}
