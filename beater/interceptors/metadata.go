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
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/elastic/apm-server/internal/netutil"
)

// ClientMetadata returns an interceptor that intercepts unary gRPC requests,
// extracts metadata relating to the gRPC client, and adds it to the context.
//
// Metadata can be extracted from context using ClientMetadataFromContext.
func ClientMetadata() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var values ClientMetadataValues
		if p, ok := peer.FromContext(ctx); ok {
			values.SourceAddr = p.Addr
			if addr, ok := p.Addr.(*net.TCPAddr); ok {
				values.ClientIP = addr.IP
			}
		}
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if ua := md["user-agent"]; len(ua) > 0 {
				values.UserAgent = ua[0]
			}
			// Account for `forwarded`, `x-real-ip`, `x-forwarded-for` headers
			if ip, _ := netutil.ClientAddrFromHeaders(http.Header(md)); ip != nil {
				values.SourceNATIP = ip
			}
		}
		ctx = context.WithValue(ctx, clientMetadataKey{}, values)
		return handler(ctx, req)
	}
}

type clientMetadataKey struct{}

// ContextWithClientMetadata returns a copy of ctx with values.
func ContextWithClientMetadata(ctx context.Context, values ClientMetadataValues) context.Context {
	return context.WithValue(ctx, clientMetadataKey{}, values)
}

// ClientMetadataFromContext returns client metadata extracted by the ClientMetadata interceptor.
func ClientMetadataFromContext(ctx context.Context) (ClientMetadataValues, bool) {
	values, ok := ctx.Value(clientMetadataKey{}).(ClientMetadataValues)
	return values, ok
}

// ClientMetadataValues holds metadata relating to the gRPC client that initiated the request being handled.
type ClientMetadataValues struct {
	// SourceAddr holds the address of the (source) network peer, if known.
	SourceAddr net.Addr

	// SourceNATIP holds the IP address of the originating gRPC client, if known,
	// as recorded in Forwarded, X-Forwarded-For, etc.
	//
	// For requests without one of the forwarded headers, this will be
	// blank.
	SourceNATIP net.IP

	// ClientIP holds the IP address of the originating gRPC client, if
	// known.
	ClientIP net.IP

	// UserAgent holds the User-Agent for the gRPC client, if known.
	UserAgent string
}
