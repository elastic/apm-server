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

package utility

import (
	"net/url"
	"path"
)

func UrlPath(p string) string {
	url, err := url.Parse(p)
	if err != nil {
		return p
	}
	return url.Path
}

func CleanUrlPath(p string) string {
	url, err := url.Parse(p)
	if err != nil {
		return p
	}
	url.Path = path.Clean(url.Path)
	return url.String()
}

// InsertInMap modifies `data` *in place*, inserting `values` at the given `key`.
// If `key` doesn't exist in data (at the top level), it gets created.
// If the value under `key` is not a map, InsertInMap does nothing.
func InsertInMap(data map[string]interface{}, key string, values map[string]interface{}) {
	if data == nil || values == nil || key == "" {
		return
	}

	if _, ok := data[key]; !ok {
		data[key] = make(map[string]interface{})
	}

	if nested, ok := data[key].(map[string]interface{}); ok {
		for newKey, newValue := range values {
			nested[newKey] = newValue
		}
	}

}

// Contains does the obvious
func Contains(s string, a []string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
