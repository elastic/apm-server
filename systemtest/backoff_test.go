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

package systemtest_test

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackOff(t *testing.T) {
	// Verify that the total time to complete the X requests with random
	// jitter takes differing amounts of time.  This is an attempt at
	// "proving" that reconnect attempts across different servers won't
	// overlap since they don't execute in the same time frame.
	var (
		totalsUniq = make(map[time.Duration]struct{})
		totals     = []time.Duration{}
		iterations = []int{1, 2, 3}
		resc       = make(chan time.Duration)
		wg         sync.WaitGroup
	)
	for range iterations {
		wg.Add(1)
		go func() {
			resc <- testBackoff(t, 10, time.Second, 2*time.Minute)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(resc)
	}()

	for total := range resc {
		totalsUniq[total] = struct{}{}
		totals = append(totals, total)
	}

	// If our map length equals our iterations length, then each test took
	// a different amount of time.
	assert.Equal(t, len(iterations), len(totalsUniq))
	// TODO: What sort of difference do we want / expect?
	want := 500 * time.Millisecond
	for i, tt := range totals {
		// Compare to every value in the slice to every other value.
		// Start comparison with next element in list, i+1
		for j := i + 1; j < len(totals); j++ {
			k := j % len(totals)
			delta := math.Abs(float64(totals[k] - tt))
			// Assert that the delta between each iteration is more than 500ms
			assert.Greaterf(t, delta, float64(want),
				fmt.Sprintf("time difference between retries %v is not greater than desired minimum delay %v", time.Duration(delta), want),
			)
		}
	}
}

func testBackoff(t *testing.T, requestCount uint64, backoffInit, backoffMax time.Duration) time.Duration {
	var (
		t0, tLast time.Time
		counter   uint64
		total     time.Duration

		waitc = make(chan struct{})
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t0.IsZero() && tLast.IsZero() {
			t0 = time.Now()
			tLast = time.Now()
		}

		// Calculate total time and time since last request.
		tLast = time.Now()
		total = tLast.Sub(t0)
		if atomic.AddUint64(&counter, 1) == requestCount {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			close(waitc)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))

	// ts.URL needs to be put in as the elasticsearch url
	tsHost, tsPort, err := net.SplitHostPort(ts.URL[7:])
	require.NoError(t, err)

	originalESHost := os.Getenv("ES_HOST")
	originalESPort := os.Getenv("ES_PORT")

	os.Setenv("ES_HOST", tsHost)
	os.Setenv("ES_PORT", tsPort)

	defer func() {
		os.Setenv("ES_HOST", originalESHost)
		os.Setenv("ES_PORT", originalESPort)
	}()

	srv := apmservertest.NewUnstartedServer(t)

	srv.Config.Output.Elasticsearch.BackoffInit = backoffInit
	srv.Config.Output.Elasticsearch.BackoffMax = backoffMax

	require.NoError(t, srv.Start())

	<-waitc
	ts.Close()
	srv.Close()
	return total
}
