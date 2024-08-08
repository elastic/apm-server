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

package modelpb

import (
	"net/url"
	"strconv"
)

func ParseURL(original, defaultHostname, defaultScheme string) *URL {
	original = truncate(original)
	url, err := url.Parse(original)
	if err != nil {
		return &URL{Original: original}
	}
	if url.Scheme == "" {
		url.Scheme = defaultScheme
		if url.Scheme == "" {
			url.Scheme = "http"
		}
	}
	if url.Host == "" {
		url.Host = defaultHostname
	}
	out := URL{}
	out.Original = original
	out.Scheme = url.Scheme
	out.Full = truncate(url.String())
	out.Domain = truncate(url.Hostname())
	out.Path = truncate(url.Path)
	out.Query = truncate(url.RawQuery)
	out.Fragment = url.Fragment
	if port := url.Port(); port != "" {
		if intv, err := strconv.Atoi(port); err == nil {
			out.Port = uint32(intv)
		}
	}
	return &out
}

// truncate returns s truncated at n runes, and the number of runes in the resulting string (<= n).
func truncate(s string) string {
	var j int
	for i := range s {
		if j == 1024 {
			return s[:i]
		}
		j++
	}
	return s
}
