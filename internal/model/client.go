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
	"net/netip"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

// Client holds information about the client of a request.
type Client struct {
	// Domain holds the client's domain (FQDN).
	Domain string

	// IP holds the client's IP address.
	IP netip.Addr

	// Port holds the client's IP port.
	Port int
}

func (c *Client) fields() mapstr.M {
	var fields mapStr
	fields.maybeSetString("domain", c.Domain)
	if c.IP.IsValid() {
		fields.set("ip", c.IP.String())
	}
	if c.Port > 0 {
		fields.set("port", c.Port)
	}
	return mapstr.M(fields)
}
