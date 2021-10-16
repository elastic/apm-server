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

package modeldecoderutil

import (
	"encoding/json"
	"net/http"

	"github.com/elastic/beats/v7/libbeat/common"
)

// HTTPHeadersToMap converts h to a common.MapStr, suitable for
// use in model.HTTP.{Request,Response}.Headers.
func HTTPHeadersToMap(h http.Header) common.MapStr {
	if len(h) == 0 {
		return nil
	}
	m := make(common.MapStr, len(h))
	for k, v := range h {
		m[k] = v
	}
	return m
}

// NormalizeHTTPRequestBody recurses through v, replacing any instance of
// a json.Number with float64.
//
// TODO(axw) define a more restrictive schema for context.request.body
// so this is unnecessary. Agents are unlikely to send numbers, but
// seeing as the schema does not prevent it we need this.
func NormalizeHTTPRequestBody(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		for i, elem := range v {
			v[i] = NormalizeHTTPRequestBody(elem)
		}
		if len(v) == 0 {
			return nil
		}
	case map[string]interface{}:
		m := v
		for k, v := range v {
			v := NormalizeHTTPRequestBody(v)
			if v != nil {
				m[k] = v
			} else {
				delete(m, k)
			}
		}
		if len(m) == 0 {
			return nil
		}
	case json.Number:
		if floatVal, err := v.Float64(); err == nil {
			return floatVal
		}
	}
	return v
}
