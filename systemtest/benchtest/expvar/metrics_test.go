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

func TestStart(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := StartNewCollector(ctx, server.URL, 10*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), c.Get(Goroutines).samples)
	assert.Equal(t, int64(1), c.Get(Goroutines).First)
}

func TestAggregate(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	<-time.After(55 * time.Millisecond)
	stats := c.Get(Goroutines)
	assert.GreaterOrEqual(t, stats.Last, int64(6))
	assert.Equal(t, stats.Last, stats.Max)
	assert.Equal(t, stats.First, stats.Min)
	assert.Equal(t, float64(stats.Last*(stats.Last+1))/float64(2*stats.samples), stats.Mean)
}

func TestWatch(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	watcher, err := c.AddWatch(Goroutines, 5)
	assert.NoError(t, err)
	select {
	case w := <-watcher:
		assert.True(t, w)
		assert.GreaterOrEqual(t, c.Get(Goroutines).Last, int64(5))
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchFalse(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	watcher, err := c.AddWatch(Goroutines, 20)
	require.NoError(t, err)
	cancel()
	select {
	case w := <-watcher:
		assert.False(t, w)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "timed out while waiting for watcher")
	}
}

func TestWatchNonBlocking(t *testing.T) {
	server := getTestServer(t)
	defer server.Close()
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, _ := StartNewCollector(ctx, server.URL, 10*time.Millisecond)
	watcher, err := c.AddWatch(Goroutines, 5)
	require.NoError(t, err)

	// Wait for the context to be cancelled.
	<-time.After(timeout)

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

func getTestServer(t *testing.T) *httptest.Server {
	var count uint64
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/debug/vars" {
				t.Errorf("unexpcted path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"beat.runtime.goroutines": %d}`, atomic.AddUint64(&count, 1))))
		}),
	)
}
