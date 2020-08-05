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

	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/esutil"
)

func (es *Client) Search(index string) *SearchRequest {
	req := &SearchRequest{es: es}
	req.Index = []string{index}
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

func (r *SearchRequest) Do(ctx context.Context, out *SearchResult, opts ...RequestOption) (*esapi.Response, error) {
	return r.es.Do(ctx, &r.SearchRequest, out, opts...)
}

type SearchResult struct {
	Hits SearchHits `json:"hits"`
}

type SearchHits struct {
	Hits []SearchHit `json:"hits"`
}

// NonEmptyCondition returns a ConditionFunc which will return true if h.Hits is non-empty.
func (h *SearchHits) NonEmptyCondition() ConditionFunc {
	return func(*esapi.Response) bool { return len(h.Hits) != 0 }
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
