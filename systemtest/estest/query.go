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

import "encoding/json"

type BoolQuery struct {
	Filter             []interface{}
	Must               []interface{}
	MustNot            []interface{}
	Should             []interface{}
	MinimumShouldMatch int
	Boost              float64
}

func (q BoolQuery) MarshalJSON() ([]byte, error) {
	type boolQuery struct {
		Filter             []interface{} `json:"filter,omitempty"`
		Must               []interface{} `json:"must,omitempty"`
		MustNot            []interface{} `json:"must_not,omitempty"`
		Should             []interface{} `json:"should,omitempty"`
		MinimumShouldMatch int           `json:"minimum_should_match,omitempty"`
		Boost              float64       `json:"boost,omitempty"`
	}
	return encodeQueryJSON("bool", boolQuery(q))
}

type TermQuery struct {
	Field string
	Value interface{}
	Boost float64
}

func (q TermQuery) MarshalJSON() ([]byte, error) {
	type termQuery struct {
		Value interface{} `json:"value"`
		Boost float64     `json:"boost,omitempty"`
	}
	return encodeQueryJSON("term", map[string]interface{}{
		q.Field: termQuery{q.Value, q.Boost},
	})
}

func encodeQueryJSON(k string, v interface{}) ([]byte, error) {
	m := map[string]interface{}{k: v}
	return json.Marshal(m)
}
