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

package modeldecoder

import "encoding/json"

func getObject(obj map[string]interface{}, key string) map[string]interface{} {
	value, _ := obj[key].(map[string]interface{})
	return value
}

func decodeString(obj map[string]interface{}, key string, out *string) bool {
	if value, ok := obj[key].(string); ok {
		*out = value
		return true
	}
	return false
}

func decodeInt(obj map[string]interface{}, key string, out *int) bool {
	var f float64
	if decodeFloat64(obj, key, &f) {
		*out = int(f)
		return true
	}
	return false
}

func decodeFloat64(obj map[string]interface{}, key string, out *float64) bool {
	switch value := obj[key].(type) {
	case json.Number:
		if f, err := value.Float64(); err == nil {
			*out = f
		}
		return true
	case float64:
		*out = value
		return true
	}
	return false
}

func safeInverse(f float64) float64 {
	if f == 0 {
		return 0
	}
	return 1 / f
}
