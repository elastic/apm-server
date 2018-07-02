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

package utility_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/utility"
)

func TestRemoteAddr(t *testing.T) {
	req := &http.Request{
		RemoteAddr: "[::1]:1234",
		Header:     make(http.Header),
	}
	assert.Equal(t, "::1", utility.RemoteAddr(req))

	req.Header.Set("X-Forwarded-For", "client.invalid")
	assert.Equal(t, "client.invalid", utility.RemoteAddr(req))

	req.Header.Set("X-Forwarded-For", "client.invalid, proxy.invalid")
	assert.Equal(t, "client.invalid", utility.RemoteAddr(req))

	req.Header.Set("X-Real-IP", "127.1.2.3")
	assert.Equal(t, "127.1.2.3", utility.RemoteAddr(req))

	// "for" is missing from Forwarded, so fall back to the next thing
	req.Header.Set("Forwarded", "")
	assert.Equal(t, "127.1.2.3", utility.RemoteAddr(req))

	req.Header.Set("Forwarded", "for=_secret")
	assert.Equal(t, "_secret", utility.RemoteAddr(req))

	req.Header.Set("Forwarded", "for=[2001:db8:cafe::17]:4711")
	assert.Equal(t, "2001:db8:cafe::17", utility.RemoteAddr(req))
}
