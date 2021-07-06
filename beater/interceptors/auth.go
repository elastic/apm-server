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
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
)

// Authenticator provides an interface for authenticating a client.
type Authenticator interface {
	Authenticate(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error)
}

// MethodAuthenticator is a function type for authenticating a gRPC method call.
// This is used to authenticate gRPC method calls by extracting authentication tokens
// from incoming context metadata or from the request payload.
type MethodAuthenticator func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error)

// Auth returns a grpc.UnaryServerInterceptor that ensures method calls are
// authenticated before passing on to the next handler.
//
// Authentication is performed using a MethodAuthenticator from the combined
// map parameters, keyed on the full gRPC method name (info.FullMethod).
// If there is no handler defined for the method, authentication fails.
func Auth(methodHandlers ...map[string]MethodAuthenticator) grpc.UnaryServerInterceptor {
	combinedMethodHandlers := make(map[string]MethodAuthenticator)
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
		authenticator, ok := combinedMethodHandlers[info.FullMethod]
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "no auth method defined for %q", info.FullMethod)
		}
		details, authz, err := authenticator(ctx, req)
		if err != nil {
			if errors.Is(err, auth.ErrAuthFailed) {
				return nil, status.Error(codes.Unauthenticated, err.Error())
			}
			return nil, err
		}
		ctx = ContextWithAuthenticationDetails(ctx, details)
		ctx = auth.ContextWithAuthorizer(ctx, authz)
		resp, err := handler(ctx, req)
		if errors.Is(err, auth.ErrUnauthorized) {
			// Processors may indicate that a request is unauthorized by returning auth.ErrUnauthorized.
			err = status.Error(codes.PermissionDenied, err.Error())
		}
		return resp, err
	}
}

// MetadataMethodAuthenticator returns a MethodAuthenticator that extracts
// authentication parameters from the "authorization" metadata in ctx,
// calling authenticator.Authenticate.
func MetadataMethodAuthenticator(authenticator Authenticator) MethodAuthenticator {
	return func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error) {
		var authHeader string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(headers.Authorization); len(values) > 0 {
				authHeader = values[0]
			}
		}
		kind, token := auth.ParseAuthorizationHeader(authHeader)
		return authenticator.Authenticate(ctx, kind, token)
	}
}

type authenticationDetailsKey struct{}

// ContextWithAuthenticationDetails returns a copy of ctx with details.
func ContextWithAuthenticationDetails(ctx context.Context, details auth.AuthenticationDetails) context.Context {
	return context.WithValue(ctx, authenticationDetailsKey{}, details)
}

// AuthenticationDetailsFromContext returns client metadata extracted by the ClientMetadata interceptor.
func AuthenticationDetailsFromContext(ctx context.Context) (auth.AuthenticationDetails, bool) {
	details, ok := ctx.Value(authenticationDetailsKey{}).(auth.AuthenticationDetails)
	return details, ok
}
