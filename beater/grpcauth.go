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

package beater

import (
	"context"
	"strings"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/headers"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// newAuthUnaryServerInterceptor returns a grpc.UnaryServerInterceptor which
// performs per-RPC auth using "Authorization" metadata for OpenTelemetry methods.
//
// TODO(axw) when we get rid of the standalone Jaeger port move Jaeger auth
// handling to here, possibly by dispatching to a callback based on the method
// name and req.
func newAuthUnaryServerInterceptor(builder *authorization.Builder) grpc.UnaryServerInterceptor {
	authHandler := builder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		if strings.HasPrefix(info.FullMethod, "/opentelemetry") {
			if err := verifyGRPCAuthorization(ctx, authHandler); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func verifyGRPCAuthorization(ctx context.Context, authHandler *authorization.Handler) error {
	var authHeader string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(headers.Authorization); len(values) > 0 {
			authHeader = values[0]
		}
	}
	auth := authHandler.AuthorizationFor(authorization.ParseAuthorizationHeader(authHeader))
	result, err := auth.AuthorizedFor(ctx, authorization.Resource{})
	if err != nil {
		return err
	}
	if !result.Authorized {
		message := "unauthorized"
		if result.Reason != "" {
			message = result.Reason
		}
		return status.Error(codes.Unauthenticated, message)
	}
	return nil
}
