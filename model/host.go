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

	"github.com/elastic/beats/v7/libbeat/common"
)

type Host struct {
	// Hostname holds the detected hostname of the host.
	Hostname string

	// Name holds the user-defined name of the host, or the
	// detected hostname.
	Name string

	// ID holds a unique ID for the host.
	ID string

	// Architecture holds the host machine architecture.
	Architecture string

	// Type holds the host type, e.g. cloud instance machine type.
	Type string

	// IP holds the IP addresses of the host.
	IP []net.IP

	// OS holds information about the operating system running on the host.
	OS OS
}

func (h *Host) fields() common.MapStr {
	if h == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("hostname", h.Hostname)
	fields.maybeSetString("name", h.Name)
	fields.maybeSetString("architecture", h.Architecture)
	fields.maybeSetString("type", h.Type)
	if len(h.IP) > 0 {
		ips := make([]string, len(h.IP))
		for i, ip := range h.IP {
			ips[i] = ip.String()
		}
		fields.set("ip", ips)
	}
	fields.maybeSetMapStr("os", h.OS.fields())
	return common.MapStr(fields)
}
