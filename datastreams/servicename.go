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

package datastreams

import "strings"

// NormalizeServiceName translates serviceName into a string suitable
// for inclusion in a data stream name.
//
// Concretely, this function will lowercase the string and replace any
// reserved characters with "_".
//
// TODO: use when Fleet supports variables in data streams
func NormalizeServiceName(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(replaceReservedRune, s)
	return s
}

func replaceReservedRune(r rune) rune {
	switch r {
	case '\\', '/', '*', '?', '"', '<', '>', '|', ' ', ',', '#', ':':
		// These characters are not permitted in data stream names
		// by Elasticsearch.
		return '_'
	case '-':
		// Hyphens are used to separate the data stream type, dataset,
		// and namespace.
		return '_'
	}
	return r
}
