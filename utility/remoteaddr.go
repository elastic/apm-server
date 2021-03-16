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
	"strings"
)

var parseHeadersInOrder = []func(http.Header) string{
	parseForwardedHeader,
	parseXRealIP,
	parseXForwardedFor,
}

// RemoteAddr returns the remote address for the HTTP request.
//
// In order:
//  - if the Forwarded header is set, then the first item in the
//    list's "for" field is used, if it exists. The "for" value
//    is returned even if it is an obfuscated identifier.
//  - if the X-Real-Ip header is set, then its value is returned.
//  - if the X-Forwarded-For header is set, then the first value
//    in the comma-separated list is returned.
//  - otherwise, the host portion of req.RemoteAddr is returned.
//
// Because the client can control the headers, they can control
// the result of this function. The result should therefore not
// necessarily be trusted to be correct.
func RemoteAddr(req *http.Request) string {
	for _, parseFn := range parseHeadersInOrder {
		if remoteAddr := parseFn(req.Header); remoteAddr != "" {
			return remoteAddr
		}
	}
	host, _ := splitHost(req.RemoteAddr)
	return host
}

func splitHost(in string) (host, port string) {
	if strings.LastIndexByte(in, ':') == -1 {
		// In the common (relative to other "errors") case that
		// there is no colon, we can avoid allocations by not
		// calling SplitHostPort.
		return in, ""
	}
	host, port, err := net.SplitHostPort(in)
	if err != nil {
		return in, ""
	}
	return host, port
}

func parseForwardedHeader(header http.Header) string {
	if fwd := getHeader(header, "Forwarded"); fwd != "" {
		forwarded := parseForwarded(fwd)
		if forwarded.For != "" {
			host, _ := splitHost(forwarded.For)
			return host
		}
	}
	return ""
}

func parseXRealIP(header http.Header) string {
	return getHeader(header, "X-Real-Ip")
}

func parseXForwardedFor(header http.Header) string {
	if xff := getHeader(header, "X-Forwarded-For"); xff != "" {
		if sep := strings.IndexRune(xff, ','); sep > 0 {
			xff = xff[:sep]
		}
		return strings.TrimSpace(xff)
	}
	return ""
}

func getHeader(header http.Header, key string) string {
	if v := header.Get(key); v != "" {
		return v
	}

	// header.Get() internally canonicalizes key names, but metadata.Pairs uses
	// lowercase keys. Using the lowercase key name allows this function to be
	// used for gRPC metadata.
	if v, ok := header[strings.ToLower(key)]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}
