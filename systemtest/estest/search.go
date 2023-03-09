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
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"
)

// ExpectDocs searches index with query, returning the results.
//
// ExpectDocs is equivalent to calling ExpectMinDocs with a minimum of 1.
func (es *Client) ExpectDocs(t testing.TB, index string, query interface{}, opts ...RequestOption) SearchResult {
	t.Helper()
	return es.ExpectMinDocs(t, 1, index, query, opts...)
}

// ExpectMinDocs searches index with query, returning the results.
//
// If the search returns fewer than min results within 10 seconds
// (by default), ExpectMinDocs will call t.Error().
func (es *Client) ExpectMinDocs(t testing.TB, min int, index string, query interface{}, opts ...RequestOption) SearchResult {
	t.Helper()
	var result SearchResult
	req := es.Search(index)
	req.ExpandWildcards = "open,hidden"
	if min > 10 {
		// Size defaults to 10. If the caller expects more than 10,
		// return it in the search so we don't have to search again.
		req = req.WithSize(min)
	}
	if query != nil {
		req = req.WithQuery(query)
	}
	opts = append(opts, WithCondition(AllCondition(
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

func (es *Client) ExpectSourcemapError(t testing.TB, index string, retry func(), query interface{}, updated bool) SearchResult {
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

			result := es.ExpectDocs(t, index, query)

			if isFetcherAvailable(t, result) {
				assertSourcemapUpdated(t, result, updated)
				return result
			}
		}
	}
}

func isFetcherAvailable(t testing.TB, result SearchResult) bool {
	t.Helper()

	for _, sh := range result.Hits.Hits {
		if bytes.Contains(sh.RawSource, []byte("metadata fetcher is not ready: fetcher unavailable")) {
			return false
		}
	}

	return true
}

func assertSourcemapUpdated(t testing.TB, result SearchResult, updated bool) {
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

func (es *Client) Search(index string) *SearchRequest {
	req := &SearchRequest{es: es}
	req.Index = strings.Split(index, ",")
	req.Body = strings.NewReader(`{"fields": ["*"]}`)
	return req
}

type SearchRequest struct {
	esapi.SearchRequest
	es *Client
}

func (r *SearchRequest) WithQuery(q interface{}) *SearchRequest {
	var body struct {
		Query  interface{} `json:"query"`
		Fields []string    `json:"fields"`
	}
	body.Query = q
	body.Fields = []string{"*"}
	r.Body = esutil.NewJSONReader(&body)
	return r
}

func (r *SearchRequest) WithSort(fieldDirection ...string) *SearchRequest {
	r.Sort = fieldDirection
	return r
}

func (r *SearchRequest) WithSize(size int) *SearchRequest {
	r.Size = &size
	return r
}

func (r *SearchRequest) Do(ctx context.Context, out *SearchResult, opts ...RequestOption) (*esapi.Response, error) {
	return r.es.Do(ctx, &r.SearchRequest, out, opts...)
}

type SearchResult struct {
	Hits         SearchHits                 `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations"`
}

type SearchHits struct {
	Total SearchHitsTotal `json:"total"`
	Hits  []SearchHit     `json:"hits"`
}

type SearchHitsTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"` // "eq" or "gte"
}

// NonEmptyCondition returns a ConditionFunc which will return true if h.Hits is non-empty.
func (h *SearchHits) NonEmptyCondition() ConditionFunc {
	return h.MinHitsCondition(1)
}

// MinHitsCondition returns a ConditionFunc which will return true if the number of h.Hits
// is at least min.
func (h *SearchHits) MinHitsCondition(min int) ConditionFunc {
	return func(*esapi.Response) bool { return len(h.Hits) >= min }
}

// TotalHitsCondition returns a ConditionFunc which will return true if the number of h.Hits
// is at least h.Total.Value. If the condition returns false, it will update req.Size to
// accommodate the number of hits in the following search.
func (h *SearchHits) TotalHitsCondition(req *SearchRequest) ConditionFunc {
	return func(*esapi.Response) bool {
		if len(h.Hits) >= h.Total.Value {
			return true
		}
		size := h.Total.Value
		req.Size = &size
		return false
	}
}

type SearchHit struct {
	Index     string
	ID        string
	Score     float64
	Fields    map[string][]interface{}
	Source    map[string]interface{}
	RawSource json.RawMessage
}

func (h *SearchHit) UnmarshalJSON(data []byte) error {
	var searchHit struct {
		Index  string                   `json:"_index"`
		ID     string                   `json:"_id"`
		Score  float64                  `json:"_score"`
		Source json.RawMessage          `json:"_source"`
		Fields map[string][]interface{} `json:"fields"`
	}
	if err := json.Unmarshal(data, &searchHit); err != nil {
		return err
	}
	h.Index = searchHit.Index
	h.ID = searchHit.ID
	h.Score = searchHit.Score
	h.RawSource = searchHit.Source
	h.Fields = searchHit.Fields
	h.Source = make(map[string]interface{})
	return json.Unmarshal(h.RawSource, &h.Source)
}

func (h *SearchHit) UnmarshalSource(out interface{}) error {
	return json.Unmarshal(h.RawSource, out)
}
