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
	"context"
	"encoding/json"
	"strings"
	"testing"

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
	if _, err := refreshReq.Do(context.Background(), es.Transport); err != nil {
		t.Fatalf("faled refreshing indices: %s: %s", index, err.Error())
	}

	if _, err := req.Do(context.Background(), &result, opts...); err != nil {
		t.Fatal(err)
	}
	return result
}

func (es *Client) Search(index string) *SearchRequest {
	req := &SearchRequest{es: es}
	req.Index = strings.Split(index, ",")
	return req
}

type SearchRequest struct {
	esapi.SearchRequest
	es *Client
}

func (r *SearchRequest) WithQuery(q interface{}) *SearchRequest {
	var body struct {
		Query interface{} `json:"query"`
	}
	body.Query = q
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
	Source    map[string]interface{}
	RawSource json.RawMessage
}

func (h *SearchHit) UnmarshalJSON(data []byte) error {
	var searchHit struct {
		Index  string          `json:"_index"`
		ID     string          `json:"_id"`
		Score  float64         `json:"_score"`
		Source json.RawMessage `json:"_source"`
	}
	if err := json.Unmarshal(data, &searchHit); err != nil {
		return err
	}
	h.Index = searchHit.Index
	h.ID = searchHit.ID
	h.Score = searchHit.Score
	h.RawSource = searchHit.Source
	h.Source = make(map[string]interface{})
	return json.Unmarshal(h.RawSource, &h.Source)
}

func (h *SearchHit) UnmarshalSource(out interface{}) error {
	return json.Unmarshal(h.RawSource, out)
}
