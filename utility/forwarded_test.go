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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseForwarded(t *testing.T) {
	type test struct {
		name   string
		header string
		expect forwardedHeader
	}

	tests := []test{{
		name:   "Forwarded",
		header: "by=127.0.0.1; for=127.1.1.1; Host=\"forwarded.invalid:443\"; proto=HTTPS",
		expect: forwardedHeader{
			For:   "127.1.1.1",
			Host:  "forwarded.invalid:443",
			Proto: "HTTPS",
		},
	}, {
		name:   "Forwarded-Multi",
		header: "host=first.invalid, host=second.invalid",
		expect: forwardedHeader{
			Host: "first.invalid",
		},
	}, {
		name:   "Forwarded-Malformed-Fields-Ignored",
		header: "what; nonsense=\"; host=first.invalid",
		expect: forwardedHeader{
			Host: "first.invalid",
		},
	}, {
		name:   "Forwarded-Trailing-Separators",
		header: "host=first.invalid;,",
		expect: forwardedHeader{
			Host: "first.invalid",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseForwarded(test.header)
			assert.Equal(t, test.expect, parsed)
		})
	}
}
