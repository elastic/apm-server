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

package model

import (
	"net/url"
	"strconv"

	"github.com/elastic/beats/v7/libbeat/common"
)

// URL describes an URL and its components
type URL struct {
	Original string
	Scheme   string
	Full     string
	Domain   string
	Port     int
	Path     string
	Query    string
	Fragment string
}

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
	out := &URL{
		Original: original,
		Scheme:   url.Scheme,
		Full:     truncate(url.String()),
		Domain:   truncate(url.Hostname()),
		Path:     truncate(url.Path),
		Query:    truncate(url.RawQuery),
		Fragment: url.Fragment,
	}
	if port := url.Port(); port != "" {
		if intv, err := strconv.Atoi(port); err == nil {
			out.Port = intv
		}
	}
	return out
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

// Fields returns common.MapStr holding transformed data for attribute url.
func (url *URL) Fields() common.MapStr {
	if url == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("full", url.Full)
	fields.maybeSetString("fragment", url.Fragment)
	fields.maybeSetString("domain", url.Domain)
	fields.maybeSetString("path", url.Path)
	if url.Port > 0 {
		fields.set("port", url.Port)
	}
	fields.maybeSetString("original", url.Original)
	fields.maybeSetString("scheme", url.Scheme)
	fields.maybeSetString("query", url.Query)
	return common.MapStr(fields)
}
