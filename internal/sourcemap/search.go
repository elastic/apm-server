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

package sourcemap

type searchOption func(map[string]interface{})

func search(opts ...searchOption) map[string]interface{} {
	m := make(map[string]interface{}, len(opts))

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func scrollID(s string) searchOption {
	return func(m map[string]interface{}) {
		m["scroll_id"] = s
	}
}

func source(s string) searchOption {
	return func(m map[string]interface{}) {
		m["_source"] = s
	}
}

func sources(s []string) searchOption {
	return func(m map[string]interface{}) {
		m["_source"] = s
	}
}

func size(i int) searchOption {
	return func(m map[string]interface{}) {
		m["size"] = 1
	}
}

func query(q map[string]interface{}) searchOption {
	return func(m map[string]interface{}) {
		m["query"] = q
	}
}

func wrap(k string, v map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{k: v}
}

func boolean(clause map[string]interface{}) map[string]interface{} {
	return wrap("bool", clause)
}

func must(clauses ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"must": clauses}
}

func term(k, v string) map[string]interface{} {
	return map[string]interface{}{"term": map[string]interface{}{k: v}}
}
