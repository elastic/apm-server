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

// +build go1.9

package apmgrpc

import (
	"regexp"
	"sync"

	"google.golang.org/grpc"
)

var (
	defaultServerRequestIgnorerOnce sync.Once
	defaultServerRequestIgnorer     RequestIgnorerFunc = IgnoreNone
)

// DefaultServerRequestIgnorer returns the default RequestIgnorer to use in
// handlers.
func DefaultServerRequestIgnorer() RequestIgnorerFunc {
	return defaultServerRequestIgnorer
}

// NewRegexpRequestIgnorer returns a RequestIgnorerFunc which matches requests'
// URLs against re. Note that for server requests, typically only Path and
// possibly RawQuery will be set, so the regular expression should take this
// into account.
func NewRegexpRequestIgnorer(re *regexp.Regexp) RequestIgnorerFunc {
	if re == nil {
		panic("re == nil")
	}
	return func(r *grpc.UnaryServerInfo) bool {
		return re.MatchString(r.FullMethod)
	}
}

// IgnoreNone is a RequestIgnorerFunc which ignores no requests.
func IgnoreNone(*grpc.UnaryServerInfo) bool {
	return false
}
