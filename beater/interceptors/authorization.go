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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/headers"
)

// MethodAuthorizationHandler is a function type for obtaining an Authorization
// for a gRPC method call. This is used to authorize gRPC method calls by extracting
// authentication tokens from incoming context metadata or from the request payload.
type MethodAuthorizationHandler func(ctx context.Context, req interface{}) authorization.Authorization

// Authorization returns a grpc.UnaryServerInterceptor that ensures method
// calls are authorized before passing on to the next handler.
//
// Authorization is performed using a MethodAuthorizationHandler from the
// combined map parameters, keyed on the full gRPC method name (info.FullMethod).
// If there is no handler defined for the method, authorization fails.
func Authorization(methodHandlers ...map[string]MethodAuthorizationHandler) grpc.UnaryServerInterceptor {
	combinedMethodHandlers := make(map[string]MethodAuthorizationHandler)
	for _, methodHandlers := range methodHandlers {
		for method, handler := range methodHandlers {
			combinedMethodHandlers[method] = handler
		}
	}
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		authHandler, ok := combinedMethodHandlers[info.FullMethod]
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "no auth method defined for %q", info.FullMethod)
		}
		auth := authHandler(ctx, req)
		authResult, err := auth.AuthorizedFor(ctx, authorization.Resource{})
		if err != nil {
			return nil, err
		} else if !authResult.Authorized {
			message := "unauthorized"
			if authResult.Reason != "" {
				message = authResult.Reason
			}
			return nil, status.Error(codes.Unauthenticated, message)
		}
		ctx = authorization.ContextWithAuthorization(ctx, auth)
		return handler(ctx, req)
	}
}

// MetadataMethodAuthorizationHandler returns a MethodAuthorizationHandler
// that extracts authentication parameters from the "authorization" metadata in ctx,
// calling authHandler.AuthorizedFor.
func MetadataMethodAuthorizationHandler(authHandler *authorization.Handler) MethodAuthorizationHandler {
	return func(ctx context.Context, req interface{}) authorization.Authorization {
		var authHeader string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(headers.Authorization); len(values) > 0 {
				authHeader = values[0]
			}
		}
		return authHandler.AuthorizationFor(authorization.ParseAuthorizationHeader(authHeader))
	}
}
