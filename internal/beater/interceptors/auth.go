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

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
)

// UnaryAuthenticator is an interface that gRPC services may implement to
// override the authentication mechanism. If this is not implemented, then
// the default method of extracting auth details from "Authorization"
// metadata will be used.
type UnaryAuthenticator interface {
	// AuthenticateUnaryCall is called for each method call with the
	// request context, request parameters, method name, and an
	// auth.Authenticator which should be called to return the final
	// auth.AuthenticationDetails and auth.Authorizer.
	AuthenticateUnaryCall(
		ctx context.Context,
		req interface{},
		fullMethod string,
		auth *auth.Authenticator,
	) (auth.AuthenticationDetails, auth.Authorizer, error)
}

// Auth returns a grpc.UnaryServerInterceptor that ensures method calls are
// authenticated before passing on to the next handler.
//
// Authentication is performed using the service's AuthenticateUnaryCall
// method, if implemented, and AuthorizationMetadataAuthenticator otherwise.
func Auth(authenticator *auth.Authenticator) grpc.UnaryServerInterceptor {
	var defaultAuthenticator UnaryAuthenticator = AuthorizationMetadataAuthenticator{}
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		unaryAuthenticator, ok := info.Server.(UnaryAuthenticator)
		if !ok {
			unaryAuthenticator = defaultAuthenticator
		}
		details, authz, err := unaryAuthenticator.AuthenticateUnaryCall(ctx, req, info.FullMethod, authenticator)
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

// AuthorizationMetadataAuthenticator is a UnaryAuthenticator which extracts
// auth details from the incoming "authorization" metadata, and passes it to
// the supplied Authenticator.
type AuthorizationMetadataAuthenticator struct{}

func (a AuthorizationMetadataAuthenticator) AuthenticateUnaryCall(
	ctx context.Context,
	req interface{},
	fullMethod string,
	authenticator *auth.Authenticator,
) (auth.AuthenticationDetails, auth.Authorizer, error) {
	var authHeader string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(headers.Authorization); len(values) > 0 {
			authHeader = values[0]
		}
	}
	kind, token := auth.ParseAuthorizationHeader(authHeader)
	return authenticator.Authenticate(ctx, kind, token)
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
