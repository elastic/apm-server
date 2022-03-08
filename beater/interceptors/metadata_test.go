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

package interceptors

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func TestClientMetadata(t *testing.T) {
	tcpAddr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 56837,
	}
	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP("1.1.1.1"),
		Port: 1111,
	}

	interceptor := ClientMetadata()

	for _, test := range []struct {
		peer     *peer.Peer
		metadata metadata.MD
		expected ClientMetadataValues
	}{{
		expected: ClientMetadataValues{},
	}, {
		peer: &peer.Peer{Addr: udpAddr},
		expected: ClientMetadataValues{
			SourceAddr: udpAddr,
		},
	}, {
		peer: &peer.Peer{Addr: tcpAddr},
		expected: ClientMetadataValues{
			SourceAddr: tcpAddr,
			ClientIP:   tcpAddr.IP,
		},
	}, {
		peer:     &peer.Peer{Addr: tcpAddr},
		metadata: metadata.Pairs("X-Real-Ip", "5.6.7.8"),
		expected: ClientMetadataValues{
			SourceAddr:  &net.TCPAddr{IP: net.ParseIP("5.6.7.8")},
			ClientIP:    net.ParseIP("5.6.7.8"),
			SourceNATIP: tcpAddr.IP,
		},
	}, {
		metadata: metadata.Pairs("User-Agent", "User-Agent"),
		expected: ClientMetadataValues{UserAgent: "User-Agent"},
	}} {
		ctx := context.Background()
		if test.peer != nil {
			ctx = peer.NewContext(ctx, test.peer)
		}
		if test.metadata != nil {
			ctx = metadata.NewIncomingContext(ctx, test.metadata)
		}
		var got ClientMetadataValues
		var ok bool
		interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			got, ok = ClientMetadataFromContext(ctx)
			return nil, nil
		})
		assert.True(t, ok)
		assert.Equal(t, test.expected, got)
	}
}
