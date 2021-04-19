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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestClientFields(t *testing.T) {
	for name, tc := range map[string]struct {
		domain string
		ip     net.IP
		port   int
		out    common.MapStr
	}{
		"Empty":  {out: nil},
		"IPv4":   {ip: net.ParseIP("192.0.0.1"), out: common.MapStr{"ip": "192.0.0.1"}},
		"IPv6":   {ip: net.ParseIP("2001:db8::68"), out: common.MapStr{"ip": "2001:db8::68"}},
		"Port":   {port: 123, out: common.MapStr{"port": 123}},
		"Domain": {domain: "testing.invalid", out: common.MapStr{"domain": "testing.invalid"}},
	} {
		t.Run(name, func(t *testing.T) {
			c := Client{
				Domain: tc.domain,
				IP:     tc.ip,
				Port:   tc.port,
			}
			assert.Equal(t, tc.out, c.fields())
		})
	}
}
