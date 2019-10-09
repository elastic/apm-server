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
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testRequest(remote, real, forward *string) *http.Request {
	req, _ := http.NewRequest("POST", "_", nil)
	if remote != nil {
		req.RemoteAddr = *remote
	}
	if real != nil {
		req.Header.Add("X-Real-IP", *real)
	}
	if forward != nil {
		req.Header.Add("X-Forwarded-For", *forward)
	}
	return req
}

func TestExtractIP(t *testing.T) {
	remote := "10.11.12.13"
	remoteWithPort := remote + ":8080"
	realIp := "54.55.101.102"
	forwardedFor := "54.56.103.104"
	forwardedForMultiple := "54.56.103.104 , 54.57.105.106 , 54.58.107.108"
	empty := ""

	testCases := []struct {
		want                  string
		remote, real, forward *string
	}{
		{realIp, &remoteWithPort, &realIp, nil},
		{realIp, &remoteWithPort, &realIp, &forwardedFor},
		{forwardedFor, &remoteWithPort, nil, &forwardedFor},
		{forwardedFor, &remoteWithPort, nil, &forwardedForMultiple},
		{remote, &remoteWithPort, nil, nil},
		{remote, &remoteWithPort, &empty, &empty},
		{empty, &empty, &empty, &empty},
	}

	nilOrString := func(v *string) string {
		if v == nil {
			return "nil"
		}
		return *v
	}
	for _, tc := range testCases {
		name := fmt.Sprintf("extractIP remote: %v, X-Real-IP: %v, X-Forwarded-For: %v",
			nilOrString(tc.remote), nilOrString(tc.real), nilOrString(tc.forward))
		req := testRequest(tc.remote, tc.real, tc.forward)
		t.Run(name, func(t *testing.T) {
			ip := ExtractIP(req)
			if tc.want == empty {
				assert.Nil(t, ip)
			} else {
				if assert.NotNil(t, ip) {
					assert.Equal(t, tc.want, ip.String())
				}
			}
		})
	}
}

func BenchmarkExtractIP(b *testing.B) {
	remote := "10.11.12.13"
	remoteWithPort := remote + ":8080"
	realIp := "54.55.101.102"
	forwardedFor := "54.56.103.104"
	forwardedForMultiple := "54.56.103.104 , 54.57.105.106 , 54.58.107.108"
	empty := ""

	testCases := []struct {
		want                  string
		remote, real, forward *string
	}{
		{realIp, &remoteWithPort, &realIp, nil},
		{realIp, &remoteWithPort, &realIp, &forwardedFor},
		{forwardedFor, &remoteWithPort, nil, &forwardedFor},
		{forwardedFor, &remoteWithPort, nil, &forwardedForMultiple},
		{remote, &remoteWithPort, nil, nil},
		{remote, &remoteWithPort, &empty, &empty},
		{empty, &empty, &empty, &empty},
	}

	nilOrString := func(v *string) string {
		if v == nil {
			return "nil"
		}
		return *v
	}
	for _, tc := range testCases {
		name := fmt.Sprintf("extractIP remote: %v, X-Real-IP: %v, X-Forwarded-For: %v",
			nilOrString(tc.remote), nilOrString(tc.real), nilOrString(tc.forward))
		req := testRequest(tc.remote, tc.real, tc.forward)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ExtractIP(req)
			}
		})
	}
}

func TestParseHost(t *testing.T) {
	for name, tc := range map[string]struct {
		inp  string
		host string
	}{
		"IPv4":         {inp: "192.0.0.1", host: "192.0.0.1"},
		"IPv4WithPort": {inp: "192.0.0.1:8080", host: "192.0.0.1"},
		"IPv6":         {inp: "2001:db8::68", host: "2001:db8::68"},
		"Invalid":      {inp: "192.0.1", host: ""},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.host, ParseHost(tc.inp))
		})
	}
}
