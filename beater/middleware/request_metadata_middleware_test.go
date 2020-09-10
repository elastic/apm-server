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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
)

func TestUserMetadataMiddleware(t *testing.T) {
	type test struct {
		remoteAddr        string
		userAgent         []string
		expectedIP        net.IP
		expectedUserAgent string
	}

	ua1 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36"
	ua2 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.14; rv:67.0) Gecko/20100101 Firefox/67.0"
	tests := []test{
		{remoteAddr: "1.2.3.4:1234", expectedIP: net.ParseIP("1.2.3.4"), userAgent: []string{ua1, ua2}, expectedUserAgent: fmt.Sprintf("%s, %s", ua1, ua2)},
		{remoteAddr: "not-an-ip:1234", userAgent: []string{ua1}, expectedUserAgent: ua1},
		{remoteAddr: ""},
	}

	for _, test := range tests {
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		c.Request.RemoteAddr = test.remoteAddr
		for _, ua := range test.userAgent {
			c.Request.Header.Add("User-Agent", ua)
		}

		Apply(UserMetadataMiddleware(), beatertest.HandlerIdle)(c)
		assert.Equal(t, test.expectedUserAgent, c.RequestMetadata.UserAgent)
		assert.Equal(t, test.expectedIP, c.RequestMetadata.ClientIP)
	}
}

func TestSystemMetadataMiddleware(t *testing.T) {
	type test struct {
		remoteAddr string
		expectedIP net.IP
	}
	tests := []test{
		{remoteAddr: "1.2.3.4:1234", expectedIP: net.ParseIP("1.2.3.4")},
		{remoteAddr: "not-an-ip:1234"},
		{remoteAddr: ""},
	}

	for _, test := range tests {
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		c.Request.RemoteAddr = test.remoteAddr

		Apply(SystemMetadataMiddleware(), beatertest.HandlerIdle)(c)
		assert.Equal(t, test.expectedIP, c.RequestMetadata.SystemIP)
	}
}
