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

package middleware

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
)

func TestUserMetadataMiddleware(t *testing.T) {
	type test struct {
		remoteAddr      string
		userAgent       []string
		expectIP        string
		expectUserAgent string
	}

	ua1 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36"
	ua2 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.14; rv:67.0) Gecko/20100101 Firefox/67.0"
	tests := []test{
		{remoteAddr: "1.2.3.4:1234", expectIP: "1.2.3.4", userAgent: []string{ua1, ua2}, expectUserAgent: fmt.Sprintf("%s, %s", ua1, ua2)},
		{remoteAddr: "not-an-ip:1234", userAgent: []string{ua1}, expectUserAgent: ua1},
		{remoteAddr: ""},
	}

	for _, test := range tests {
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		c.Request.RemoteAddr = test.remoteAddr
		for _, ua := range test.userAgent {
			c.Request.Header.Add("User-Agent", ua)
		}

		Apply(UserMetadataMiddleware(), beatertest.HandlerIdle)(c)
		expect := map[string]interface{}{
			"user-agent": test.expectUserAgent, // even if empty value
		}
		if test.expectIP != "" {
			expect["ip"] = test.expectIP
		}
		assert.Equal(t, map[string]interface{}{"user": expect}, c.RequestMetadata)
	}
}

func TestSystemMetadataMiddleware(t *testing.T) {
	type test struct {
		remoteAddr string
		expectIP   string
	}
	tests := []test{
		{remoteAddr: "1.2.3.4:1234", expectIP: "1.2.3.4"},
		{remoteAddr: "not-an-ip:1234"},
		{remoteAddr: ""},
	}

	for _, test := range tests {
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		c.Request.RemoteAddr = test.remoteAddr

		Apply(SystemMetadataMiddleware(), beatertest.HandlerIdle)(c)
		if test.expectIP == "" {
			assert.Empty(t, c.RequestMetadata)
		} else {
			assert.Equal(t, map[string]interface{}{
				"system": map[string]interface{}{
					"ip": test.expectIP,
				},
			}, c.RequestMetadata)
		}
	}
}
