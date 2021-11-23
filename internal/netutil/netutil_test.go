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

package netutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	headerForwarded     = "Forwarded"
	headerXForwardedFor = "X-Forwarded-For"
	headerXRealIP       = "X-Real-Ip"
)

func TestClientAddrFromHeaders(t *testing.T) {
	for name, tc := range map[string]struct {
		header http.Header
		ip     string
		port   uint16
	}{
		"no header": {},
		"Invalid-X-Forwarded-For": {
			header: http.Header{headerXForwardedFor: []string{"client.invalid"}},
		},
		"Invalid-X-Real-IP-Invalid": {
			header: http.Header{headerXRealIP: []string{"client.invalid"}},
		},
		"Invalid-Forwarded": {
			header: http.Header{headerForwarded: []string{"for=client.invalid"}},
		},
		"Invalid-ForwardedMissingFor": {
			header: http.Header{headerForwarded: []string{"128.0.0.5"}},
		},
		"X-Forwarded-For": {
			header: http.Header{headerXForwardedFor: []string{"123.0.0.1"}},
			ip:     "123.0.0.1",
		},
		"X-Forwarded-For-First": {
			header: http.Header{headerXForwardedFor: []string{"123.0.0.1, 127.0.0.1"}},
			ip:     "123.0.0.1",
		},
		"X-Real-IP": {
			header: http.Header{headerXRealIP: []string{"123.0.0.1:6060"}},
			ip:     "123.0.0.1",
			port:   6060,
		},
		"X-Real-IP-Fallback": {
			header: http.Header{headerXRealIP: []string{"invalid"}, headerXForwardedFor: []string{"182.0.0.9"}},
			ip:     "182.0.0.9",
		},
		"Forwarded": {
			header: http.Header{headerForwarded: []string{"for=[2001:db8:cafe::17]:4711"}},
			ip:     "2001:db8:cafe::17",
			port:   4711,
		},
		"Forwarded-Fallback": {
			header: http.Header{headerForwarded: []string{"invalid"}, headerXForwardedFor: []string{"182.0.0.9"}},
			ip:     "182.0.0.9",
		},
		"Forwarded-Fallback2": {
			header: http.Header{headerForwarded: []string{"invalid"}, headerXRealIP: []string{"182.0.0.9"}},
			ip:     "182.0.0.9",
		},
		"gRPC Metadata": {
			// metadata.Pairs stores keys as lowercase
			header: http.Header{"x-real-ip": []string{"182.0.0.9"}},
			ip:     "182.0.0.9",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ip, port := ClientAddrFromHeaders(tc.header)
			if tc.ip == "" {
				assert.Nil(t, ip)
			} else {
				require.NotNil(t, ip)
				assert.Equal(t, tc.ip, ip.String())
			}
			assert.Equal(t, tc.port, port)
		})
	}
}

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
