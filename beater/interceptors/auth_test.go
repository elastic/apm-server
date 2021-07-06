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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/interceptors"
)

func TestAuth(t *testing.T) {
	type contextKey struct{}
	origContext := context.WithValue(context.Background(), contextKey{}, 123)
	origReq := "grpc_request"
	origResp := "grpc_response"
	origErr := errors.New("handler error")

	authenticated := authenticatorFunc(func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		assert.Equal(t, 123, ctx.Value(contextKey{}))
		return auth.AuthenticationDetails{Method: auth.MethodSecretToken}, auth.AnonymousAuth{}, nil
	})
	authFailed := authenticatorFunc(func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return auth.AuthenticationDetails{}, nil, auth.ErrAuthFailed
	})
	authError := authenticatorFunc(func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return auth.AuthenticationDetails{}, nil, errors.New("error occurred while authenticating")
	})

	makeMethodAuthenticator := func(authenticator interceptors.Authenticator) interceptors.MethodAuthenticator {
		return func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error) {
			require.Equal(t, origReq, req)
			return authenticator.Authenticate(ctx, "", "")
		}
	}

	interceptor := interceptors.Auth(
		map[string]interceptors.MethodAuthenticator{
			"authenticated": makeMethodAuthenticator(authenticated),
		},
		map[string]interceptors.MethodAuthenticator{
			"auth_failed": makeMethodAuthenticator(authFailed),
			"auth_error":  makeMethodAuthenticator(authError),
		},
	)

	type test struct {
		method     string
		expectResp interface{}
		expectErr  error
	}
	for _, test := range []test{{
		method:     "authenticated",
		expectResp: origResp,
		expectErr:  origErr,
	}, {
		method:    "auth_failed",
		expectErr: status.Error(codes.Unauthenticated, auth.ErrAuthFailed.Error()),
	}, {
		method:    "auth_error",
		expectErr: errors.New("error occurred while authenticating"),
	}} {
		t.Run(test.method, func(t *testing.T) {
			next := func(ctx context.Context, req interface{}) (interface{}, error) {
				return origResp, origErr
			}
			resp, err := interceptor(origContext, origReq, &grpc.UnaryServerInfo{FullMethod: test.method}, next)
			assert.Equal(t, test.expectErr, err)
			assert.Equal(t, test.expectResp, resp)
		})
	}
}

func TestAuthUnauthorized(t *testing.T) {
	var authorizer authorizerFunc = func(context.Context, auth.Action, auth.Resource) error {
		return fmt.Errorf("%w: none shall pass", auth.ErrUnauthorized)
	}
	interceptor := interceptors.Auth(
		map[string]interceptors.MethodAuthenticator{
			"method": interceptors.MethodAuthenticator(
				func(ctx context.Context, req interface{}) (auth.AuthenticationDetails, auth.Authorizer, error) {
					return auth.AuthenticationDetails{}, authorizer, nil
				},
			),
		},
	)
	next := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{})
	}
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "method"}, next)
	assert.Equal(t, status.Error(codes.PermissionDenied, "unauthorized: none shall pass"), err)
}

func TestMetadataMethodAuthenticator(t *testing.T) {
	expectDetails := auth.AuthenticationDetails{
		Method: auth.MethodSecretToken,
	}
	var expectAuthz struct {
		auth.Authorizer
	}
	var authenticator authenticatorFunc = func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return expectDetails, expectAuthz, nil
	}
	methodAuthenticator := interceptors.MetadataMethodAuthenticator(authenticator)

	ctx := context.Background()
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer abc123"))
	details, authz, err := methodAuthenticator(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectDetails, details)
	assert.Exactly(t, expectAuthz, authz)
}

type authenticatorFunc func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
	return f(ctx, kind, token)
}

type authorizerFunc func(ctx context.Context, action auth.Action, resource auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}
