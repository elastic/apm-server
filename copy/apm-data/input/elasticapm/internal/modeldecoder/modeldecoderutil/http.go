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

	"github.com/elastic/apm-data/model/modelpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// HTTPHeadersToStructPb converts h to a *structpb.Struct, suitable for
// use in modelpb.HTTP.{Request,Response}.Headers.
func HTTPHeadersToStructPb(h http.Header) *structpb.Struct {
	if len(h) == 0 {
		return nil
	}
	m := make(map[string]any, len(h))
	for k, v := range h {
		arr := make([]any, 0, len(v))
		for _, s := range v {
			arr = append(arr, s)
		}
		m[k] = arr
	}
	if str, err := structpb.NewStruct(m); err == nil {
		return str
	}
	return nil
}

func HTTPHeadersToModelpb(h http.Header, out []*modelpb.HTTPHeader) []*modelpb.HTTPHeader {
	if len(h) == 0 {
		return nil
	}
	out = ResliceAndPopulateNil(out, len(h), modelpb.HTTPHeaderFromVTPool)
	i := 0
	for k, v := range h {
		out[i].Key = k
		out[i].Value = v
		i++
	}
	return out
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
