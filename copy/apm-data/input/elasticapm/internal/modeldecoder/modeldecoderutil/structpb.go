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

	"google.golang.org/protobuf/types/known/structpb"
)

func ToStruct(m map[string]any) *structpb.Struct {
	nm := normalizeMap(m)
	if len(nm) == 0 {
		return nil
	}

	if str, err := structpb.NewStruct(nm); err == nil {
		return str
	}
	return nil
}

func ToValue(a any) *structpb.Value {
	nv := normalizeValue(a)
	if nv == nil {
		return nil
	}

	if v, err := structpb.NewValue(nv); err == nil {
		return v
	}
	return nil
}

func normalizeMap(m map[string]any) map[string]any {
	if v := normalizeValue(m); v != nil {
		return v.(map[string]any)
	}
	return nil
}

func normalizeValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		for i, elem := range v {
			v[i] = normalizeValue(elem)
		}
		if len(v) == 0 {
			return nil
		}
	case map[string]interface{}:
		m := v
		for k, v := range v {
			v := normalizeValue(v)
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
