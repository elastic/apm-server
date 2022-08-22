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

// Source holds information about the source of a network exchange.
type Source struct {
	// Domain holds the client's domain (FQDN).
	Domain string

	// IP holds the client's IP address.
	IP netip.Addr

	// Port holds the client's IP port.
	Port int

	// NAT holds the translated source based NAT sessions.
	NAT *NAT
}

func (s *Source) fields() mapstr.M {
	var fields mapStr
	fields.maybeSetString("domain", s.Domain)
	if s.IP.IsValid() {
		fields.set("ip", s.IP.String())
	}
	if s.Port > 0 {
		fields.set("port", s.Port)
	}
	if s.NAT != nil {
		if nat := s.NAT.fields(); len(nat) > 0 {
			fields.set("nat", nat)
		}
	}
	return mapstr.M(fields)
}

// NAT holds information about the translated source of a network exchange.
type NAT struct {
	// IP holds the translated IP address.
	IP netip.Addr
}

func (n *NAT) fields() mapstr.M {
	var fields mapStr
	if n.IP.IsValid() {
		fields.set("ip", n.IP.String())
	}
	return mapstr.M(fields)
}
