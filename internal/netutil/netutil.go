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

package netutil

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// ClientAddrFromHeaders returns the IP address, and optionally port, of the client
// for an HTTP request from one of various headers in order: Forwarded, X-Real-IP,
// X-Forwarded-For. For the multi-valued Forwarded and X-Forwarded-For headers, the
// first value in the list is returned. If the port is unknown, it will be zero.
//
// If the client is able to control the headers, they can control the result of this
// function. The result should therefore not necessarily be trusted to be correct;
// that depends on the presence and configuration of proxies in front of apm-server.
func ClientAddrFromHeaders(header http.Header) (ip net.IP, port uint16) {
	for _, parse := range parseHeadersInOrder {
		if ip, port := parse(header); ip != nil {
			return ip, port
		}
	}
	return nil, 0
}

var parseHeadersInOrder = []func(http.Header) (net.IP, uint16){
	parseForwardedHeader,
	parseXRealIP,
	parseXForwardedFor,
}

func parseForwardedHeader(header http.Header) (net.IP, uint16) {
	forwarded := parseForwarded(getHeader(header, "Forwarded", "forwarded"))
	if forwarded.For == "" {
		return nil, 0
	}
	return ParseIPPort(MaybeSplitHostPort(forwarded.For))
}

func parseXRealIP(header http.Header) (net.IP, uint16) {
	return ParseIPPort(MaybeSplitHostPort(getHeader(header, "X-Real-Ip", "x-real-ip")))
}

func parseXForwardedFor(header http.Header) (net.IP, uint16) {
	if xff := getHeader(header, "X-Forwarded-For", "x-forwarded-for"); xff != "" {
		if sep := strings.IndexRune(xff, ','); sep > 0 {
			xff = xff[:sep]
		}
		return ParseIPPort(MaybeSplitHostPort(strings.TrimSpace(xff)))
	}
	return nil, 0
}

func getHeader(header http.Header, key, keyLower string) string {
	if v := header.Get(key); v != "" {
		return v
	}

	// header.Get() internally canonicalizes key names, but metadata.Pairs uses
	// lowercase keys. Using the lowercase key name allows this function to be
	// used for gRPC metadata.
	if v, ok := header[keyLower]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// forwardedHeader holds information extracted from a "Forwarded" HTTP header.
type forwardedHeader struct {
	For   string
	Host  string
	Proto string
}

// parseForwarded parses a "Forwarded" HTTP header.
func parseForwarded(f string) forwardedHeader {
	// We only consider the first value in the sequence,
	// if there are multiple. Disregard everything after
	// the first comma.
	if comma := strings.IndexRune(f, ','); comma != -1 {
		f = f[:comma]
	}
	var result forwardedHeader
	for f != "" {
		field := f
		if semi := strings.IndexRune(f, ';'); semi != -1 {
			field = f[:semi]
			f = f[semi+1:]
		} else {
			f = ""
		}
		eq := strings.IndexRune(field, '=')
		if eq == -1 {
			// Malformed field, ignore.
			continue
		}
		key := strings.TrimSpace(field[:eq])
		value := strings.TrimSpace(field[eq+1:])
		if len(value) > 0 && value[0] == '"' {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				// Malformed, ignore
				continue
			}
		}
		switch strings.ToLower(key) {
		case "for":
			result.For = value
		case "host":
			result.Host = value
		case "proto":
			result.Proto = value
		}
	}
	return result
}

// ParseIPPort parses h as an IP and, if successful and p is non-empty, p as a port.
// If h cannot be parsed as an IP or p is non-empty and cannot be parsed as a port,
// ParseIPPort will return (nil, 0). If p is empty, 0 will be returned for the port.
func ParseIPPort(h, p string) (net.IP, uint16) {
	ip := net.ParseIP(h)
	if ip == nil {
		return nil, 0
	}
	if p == "" {
		return ip, 0
	}
	port, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return nil, 0
	}
	return ip, uint16(port)
}

// MaybeSplitHostPort returns the result of net.SplitHostPort if it
// would return without an error, otherwise it returns (in, ""). This
// can be used when splitting a string which may be either a host, or
// host:port pair.
func MaybeSplitHostPort(in string) (host, port string) {
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
