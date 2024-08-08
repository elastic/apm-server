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

package modeljson

import (
	"testing"

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestHTTPToModelJSON(t *testing.T) {
	headers := randomHTTPHeaders(t)
	headers2 := randomHTTPHeaders(t)
	cookies := randomKv(t)
	envs := randomKv(t)
	tru := true

	testCases := map[string]struct {
		proto    *modelpb.HTTP
		expected *modeljson.HTTP
	}{
		"empty": {
			proto:    &modelpb.HTTP{},
			expected: &modeljson.HTTP{},
		},
		"request": {
			proto: &modelpb.HTTP{
				Request: &modelpb.HTTPRequest{
					Headers:  headers,
					Env:      envs,
					Cookies:  cookies,
					Id:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
			},
			expected: &modeljson.HTTP{
				Request: &modeljson.HTTPRequest{
					Headers:  headers,
					Env:      envs,
					Cookies:  cookies,
					ID:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
			},
		},
		"response": {
			proto: &modelpb.HTTP{
				Response: &modelpb.HTTPResponse{
					Headers:         headers2,
					Finished:        &tru,
					HeadersSent:     &tru,
					TransferSize:    uint64Ptr(1),
					EncodedBodySize: uint64Ptr(2),
					DecodedBodySize: uint64Ptr(3),
					StatusCode:      200,
				},
			},
			expected: &modeljson.HTTP{
				Response: &modeljson.HTTPResponse{
					Finished:        &tru,
					HeadersSent:     &tru,
					TransferSize:    uint64Ptr(1),
					EncodedBodySize: uint64Ptr(2),
					DecodedBodySize: uint64Ptr(3),
					Headers:         headers2,
					StatusCode:      200,
				},
			},
		},
		"no pointers": {
			proto: &modelpb.HTTP{
				Request: &modelpb.HTTPRequest{
					Id:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
				Response: &modelpb.HTTPResponse{
					StatusCode: 200,
				},
				Version: "version",
			},
			expected: &modeljson.HTTP{
				Request: &modeljson.HTTPRequest{
					ID:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
				Response: &modeljson.HTTPResponse{
					StatusCode: 200,
				},
				Version: "version",
			},
		},
		"full": {
			proto: &modelpb.HTTP{
				Request: &modelpb.HTTPRequest{
					Headers:  headers,
					Env:      envs,
					Cookies:  cookies,
					Id:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
				Response: &modelpb.HTTPResponse{
					Headers:         headers2,
					Finished:        &tru,
					HeadersSent:     &tru,
					TransferSize:    uint64Ptr(1),
					EncodedBodySize: uint64Ptr(2),
					DecodedBodySize: uint64Ptr(3),
					StatusCode:      200,
				},
				Version: "version",
			},
			expected: &modeljson.HTTP{
				Request: &modeljson.HTTPRequest{
					Headers:  headers,
					Env:      envs,
					Cookies:  cookies,
					ID:       "id",
					Method:   "method",
					Referrer: "referrer",
				},
				Response: &modeljson.HTTPResponse{
					Finished:        &tru,
					HeadersSent:     &tru,
					TransferSize:    uint64Ptr(1),
					EncodedBodySize: uint64Ptr(2),
					DecodedBodySize: uint64Ptr(3),
					Headers:         headers2,
					StatusCode:      200,
				},
				Version: "version",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			out := modeljson.HTTP{
				Request:  &modeljson.HTTPRequest{},
				Response: &modeljson.HTTPResponse{},
			}
			HTTPModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out, protocmp.Transform())
			require.Empty(t, diff)
		})
	}
}
