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

package modelpb

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseIP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		address string

		expectedIP  *IP
		expectedErr string
	}{
		{
			name:    "with a valid IPv4 address",
			address: "127.0.0.1",

			expectedIP: &IP{
				V4: 2130706433,
			},
		},
		{
			name:    "with a valid IPv6 address",
			address: "::1",

			expectedIP: &IP{
				V6: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
		},
		{
			name:    "with an invalid address",
			address: "hello_world",

			expectedIP:  nil,
			expectedErr: "ParseAddr(\"hello_world\"): unable to parse IP",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := ParseIP(tt.address)

			if tt.expectedErr != "" {
				assert.Equal(t, tt.expectedErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestMustParseIP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		address string

		expectedIP    *IP
		expectedPanic bool
	}{
		{
			name:    "with a valid IPv4 address",
			address: "127.0.0.1",

			expectedIP: &IP{
				V4: 2130706433,
			},
		},
		{
			name:    "with a valid IPv6 address",
			address: "::1",

			expectedIP: &IP{
				V6: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
		},
		{
			name:    "with an invalid address",
			address: "hello_world",

			expectedIP:    nil,
			expectedPanic: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {

			fn := assert.Panics
			if !tt.expectedPanic {
				fn = assert.NotPanics
			}

			fn(t, func() {
				ip := MustParseIP(tt.address)
				assert.Equal(t, tt.expectedIP, ip)
			})
		})
	}
}

func TestAddr2IP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		address netip.Addr

		expectedIP *IP
	}{
		{
			name:    "IPv4 address 127.0.0.1",
			address: netip.MustParseAddr("127.0.0.1"),

			expectedIP: MustParseIP("127.0.0.1"),
		},
		{
			name:    "IPv6 address 0.0.0.0",
			address: netip.MustParseAddr("0.0.0.0"),

			expectedIP: MustParseIP("0.0.0.0"),
		},
		{
			name:    "IPv6 address ::1",
			address: netip.MustParseAddr("::1"),

			expectedIP: MustParseIP("::1"),
		},
		{
			name:    "IPv6 address ::",
			address: netip.MustParseAddr("::"),

			expectedIP: MustParseIP("::"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ip := Addr2IP(tt.address)

			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestIP2Addr(t *testing.T) {
	for _, tt := range []struct {
		name string
		ip   *IP

		expectedAddr netip.Addr
	}{
		{
			name: "IPv4 address 127.0.0.1",
			ip:   MustParseIP("127.0.0.1"),

			expectedAddr: netip.MustParseAddr("127.0.0.1"),
		},
		{
			name: "IPv4 address 0.0.0.0",
			ip:   MustParseIP("0.0.0.0"),

			expectedAddr: netip.MustParseAddr("0.0.0.0"),
		},
		{
			name: "IPv6 address ::1",
			ip:   MustParseIP("::1"),

			expectedAddr: netip.MustParseAddr("::1"),
		},
		{
			name: "IPv6 address ::",
			ip:   MustParseIP("::"),

			expectedAddr: netip.MustParseAddr("::"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			addr := IP2Addr(tt.ip)

			assert.Equal(t, tt.expectedAddr, addr)
		})
	}
}

func TestIP2String(t *testing.T) {
	for _, tt := range []struct {
		name string
		ip   *IP

		expectedString string
	}{
		{
			name: "with an IPv4 address",
			ip:   MustParseIP("127.0.0.1"),

			expectedString: "127.0.0.1",
		},
		{
			name: "with an IPv6 address",
			ip:   MustParseIP("::1"),

			expectedString: "::1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			str := IP2String(tt.ip)

			assert.Equal(t, tt.expectedString, str)
		})
	}
}
