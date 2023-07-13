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

package estest

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-tools/pkg/espoll"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func ExpectDocs(t testing.TB, es *espoll.Client, index string, query interface{}, opts ...espoll.RequestOption) espoll.SearchResult {
	t.Helper()
	return ExpectMinDocs(t, es, 1, index, query, opts...)
}

// ExpectMinDocs searches index with query, returning the results.
//
// If the search returns fewer than min results within 10 seconds
// (by default), ExpectMinDocs will call t.Error().
func ExpectMinDocs(t testing.TB, es *espoll.Client, min int, index string, query interface{}, opts ...espoll.RequestOption) espoll.SearchResult {
	t.Helper()
	var result espoll.SearchResult
	req := es.NewSearchRequest(index)
	req.ExpandWildcards = "open,hidden"
	if min > 10 {
		// Size defaults to 10. If the caller expects more than 10,
		// return it in the search so we don't have to search again.
		req = req.WithSize(min)
	}
	if query != nil {
		req = req.WithQuery(query)
	}
	opts = append(opts, espoll.WithCondition(espoll.AllCondition(
		result.Hits.MinHitsCondition(min),
		result.Hits.TotalHitsCondition(req),
	)))

	// Refresh the indices before issuing the search request.
	refreshReq := esapi.IndicesRefreshRequest{
		Index:           strings.Split(",", index),
		ExpandWildcards: "all",
	}
	rsp, err := refreshReq.Do(context.Background(), es.Transport)
	if err != nil {
		t.Fatalf("failed refreshing indices: %s: %s", index, err.Error())
	}

	rsp.Body.Close()

	if _, err := req.Do(context.Background(), &result, opts...); err != nil {
		t.Fatal(err)
	}
	return result
}

func ExpectSourcemapError(t testing.TB, es *espoll.Client, index string, retry func(), query interface{}, updated bool) espoll.SearchResult {
	t.Helper()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timed out while querying es")
		case <-ticker.C:
			rsp, err := es.Do(context.Background(), &esapi.IndicesDeleteDataStreamRequest{
				Name:            []string{index},
				ExpandWildcards: "all",
			}, nil)
			require.NoError(t, err)
			require.NoError(t, rsp.Body.Close())
			require.Equal(t, http.StatusOK, rsp.StatusCode)

			retry()

			result := ExpectDocs(t, es, index, query)

			if isFetcherAvailable(t, result) {
				assertSourcemapUpdated(t, result, updated)
				return result
			}
		}
	}
}

func isFetcherAvailable(t testing.TB, result espoll.SearchResult) bool {
	t.Helper()

	for _, sh := range result.Hits.Hits {
		if bytes.Contains(sh.RawSource, []byte("metadata fetcher is not ready: fetcher unavailable")) {
			return false
		}
	}

	return true
}

func assertSourcemapUpdated(t testing.TB, result espoll.SearchResult, updated bool) {
	t.Helper()

	type StacktraceFrame struct {
		Sourcemap struct {
			Updated bool
		}
	}
	type Error struct {
		Exception []struct {
			Stacktrace []StacktraceFrame
		}
		Log struct {
			Stacktrace []StacktraceFrame
		}
	}

	for _, hit := range result.Hits.Hits {
		var source struct {
			Error Error
		}
		err := hit.UnmarshalSource(&source)
		require.NoError(t, err)

		for _, exception := range source.Error.Exception {
			for _, stacktrace := range exception.Stacktrace {
				assert.Equal(t, updated, stacktrace.Sourcemap.Updated)
			}
		}

		for _, stacktrace := range source.Error.Log.Stacktrace {
			assert.Equal(t, updated, stacktrace.Sourcemap.Updated)
		}
	}
}
