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
	"net"
	"net/http"
)

// ExtractIP calls RemoteAddr(r), and passes the result into net.ParseIP
// and returns that. If the request does not have a valid IP remote address,
// this function will return nil.
func ExtractIP(r *http.Request) net.IP {
	return net.ParseIP(RemoteAddr(r))
}

func ParseHost(inp string) string {
	if ip := net.ParseIP(inp); ip != nil {
		return ip.String()
	}
	if host, _, err := net.SplitHostPort(inp); err == nil {
		return host
	}
	return ""
}
