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

// Destination holds information about the destination of a request.
type Destination struct {
	Address string
	Port    int
}

func (d *Destination) fields() common.MapStr {
	var fields mapStr
	if fields.maybeSetString("address", d.Address) {
		// Copy destination.address to destination.ip if it's a valid IP.
		//
		// TODO(axw) move this to a "convert" ingest processor once we
		// have a high enough minimum supported Elasticsearch version.
		if ip := net.ParseIP(d.Address); ip != nil {
			fields.set("ip", d.Address)
		}
	}
	if d.Port > 0 {
		fields.set("port", d.Port)
	}
	return common.MapStr(fields)
}
