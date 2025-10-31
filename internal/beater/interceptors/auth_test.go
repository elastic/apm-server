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

package interceptors_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestUnaryAuthenticator(t *testing.T) {
	type contextKey struct{}
	origContext := context.WithValue(context.Background(), contextKey{}, 123)
	origReq := "grpc_request"
	origResp := "grpc_response"
	origErr := errors.New("handler error")

	authenticator := &auth.Authenticator{}
	interceptor := interceptors.Auth(authenticator)

	call := func(t *testing.T, authnErr, authzErr error) (interface{}, error) {
		details := auth.AuthenticationDetails{
			Method: auth.MethodAPIKey,
			APIKey: &auth.APIKeyAuthenticationDetails{ID: "whatever"},
		}
		var authz authorizerFunc = func(context.Context, auth.Action, auth.Resource) error {
			return authzErr
		}

		var authFunc unaryAuthenticatorFunc = func(
			ctx context.Context,
			req interface{},
			fullMethod string,
			authenticatorArg *auth.Authenticator,
		) (auth.AuthenticationDetails, auth.Authorizer, error) {
			// The authenticator argument should be the exact
			// same as the one passed to interceptors.Auth.
			assert.Equal(t, authenticator, authenticatorArg)
			assert.Equal(t, "the_method_name", fullMethod)
			return details, authz, authnErr
		}

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			assert.Equal(t, 123, ctx.Value(contextKey{}))
			ctxDetails, ok := interceptors.AuthenticationDetailsFromContext(ctx)
			assert.True(t, ok)
			assert.Equal(t, details, ctxDetails)
			if err := auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{}); err != nil {
				return nil, err
			}
			return origResp, origErr
		}
		return interceptor(origContext, origReq, &grpc.UnaryServerInfo{
			FullMethod: "the_method_name",
			Server:     authFunc, // Server implements UnaryAuthenticator
		}, handler)
	}

	t.Run("authenticated", func(t *testing.T) {
		resp, err := call(t, nil, nil)
		assert.Equal(t, origResp, resp)
		assert.Equal(t, origErr, err)
	})

	t.Run("auth_failed", func(t *testing.T) {
		resp, err := call(t, auth.ErrAuthFailed, nil)
		assert.Nil(t, resp)
		assert.Equal(t, status.Error(codes.Unauthenticated, auth.ErrAuthFailed.Error()), err)
	})

	t.Run("auth_error", func(t *testing.T) {
		resp, err := call(t, errors.New("some other error"), nil)
		assert.Nil(t, resp)
		assert.Equal(t, errors.New("some other error"), err)
	})

	t.Run("unauthorized", func(t *testing.T) {
		authzErr := fmt.Errorf("%w: none shall pass", auth.ErrUnauthorized)
		resp, err := call(t, nil, authzErr)
		assert.Nil(t, resp)
		assert.Equal(t, status.Error(codes.PermissionDenied, authzErr.Error()), err)
	})

	t.Run("authorization_error", func(t *testing.T) {
		resp, err := call(t, nil, errors.New("some other error"))
		assert.Nil(t, resp)
		assert.Equal(t, errors.New("some other error"), err)
	})
}

func TestAuthorizationMetadataAuthenticator(t *testing.T) {
	authenticator, err := auth.NewAuthenticator(config.AgentAuth{SecretToken: "abc123"}, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	interceptor := interceptors.Auth(authenticator)

	ctx := context.Background()
	authContext := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer abc123"))

	// Call with a valid authorization header.
	resp, err := interceptor(authContext, nil, &grpc.UnaryServerInfo{
		Server: nil, // Server does not implement UnaryAuthenticator
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		details, ok := interceptors.AuthenticationDetailsFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, auth.MethodSecretToken, details.Method)
		return 123, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 123, resp)

	// Call without an authorization header, showing that authentication
	// fails and the handler is never invoked.
	resp, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{
		Server: nil, // Server does not implement UnaryAuthenticator
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("unexpected")
	})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Nil(t, resp)
}

type unaryAuthenticatorFunc func(
	ctx context.Context,
	req interface{},
	fullMethod string,
	authenticator *auth.Authenticator,
) (auth.AuthenticationDetails, auth.Authorizer, error)

func (f unaryAuthenticatorFunc) AuthenticateUnaryCall(
	ctx context.Context,
	req interface{},
	fullMethod string,
	authenticator *auth.Authenticator,
) (auth.AuthenticationDetails, auth.Authorizer, error) {
	return f(ctx, req, fullMethod, authenticator)
}

type authorizerFunc func(ctx context.Context, action auth.Action, resource auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}
