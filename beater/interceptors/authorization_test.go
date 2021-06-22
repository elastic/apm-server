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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
)

func TestAuthorization(t *testing.T) {
	type contextKey struct{}
	origContext := context.WithValue(context.Background(), contextKey{}, 123)
	origReq := "grpc_request"
	origResp := "grpc_response"
	origErr := errors.New("handler error")

	authorizedResult := authorization.Result{Authorized: true}
	anonymousResult := authorization.Result{Authorized: true, Anonymous: true}
	unauthorizedResult := authorization.Result{Authorized: false, Reason: "no particular reason"}

	authorized := authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
		assert.Equal(t, 123, ctx.Value(contextKey{}))
		return authorizedResult, nil
	})
	anonymous := authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
		assert.Equal(t, 123, ctx.Value(contextKey{}))
		return anonymousResult, nil
	})
	unauthorized := authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
		assert.Equal(t, 123, ctx.Value(contextKey{}))
		return unauthorizedResult, nil
	})
	authError := authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
		assert.Equal(t, 123, ctx.Value(contextKey{}))
		return authorization.Result{}, errors.New("error checking authorization")
	})

	makeMethodAuthorizationHandler := func(auth authorization.Authorization) interceptors.MethodAuthorizationHandler {
		return func(ctx context.Context, req interface{}) authorization.Authorization {
			require.Equal(t, origReq, req)
			return auth
		}
	}

	interceptor := interceptors.Authorization(
		map[string]interceptors.MethodAuthorizationHandler{
			"authorized": makeMethodAuthorizationHandler(authorized),
			"anonymous":  makeMethodAuthorizationHandler(anonymous),
		},
		map[string]interceptors.MethodAuthorizationHandler{
			"unauthorized": makeMethodAuthorizationHandler(unauthorized),
			"authError":    makeMethodAuthorizationHandler(authError),
		},
	)

	type test struct {
		method       string
		expectResult authorization.Result
		expectResp   interface{}
		expectErr    error
	}
	for _, test := range []test{{
		method:       "authorized",
		expectResult: authorizedResult,
		expectResp:   origResp,
		expectErr:    origErr,
	}, {
		method:       "anonymous",
		expectResult: anonymousResult,
		expectResp:   origResp,
		expectErr:    origErr,
	}, {
		method:     "unauthorized",
		expectResp: nil,
		expectErr:  status.Error(codes.Unauthenticated, "no particular reason"),
	}, {
		method:     "authError",
		expectResp: nil,
		expectErr:  errors.New("error checking authorization"),
	}} {
		t.Run(test.method, func(t *testing.T) {
			var authorizedForResult authorization.Result
			var authorizedForErr error
			next := func(ctx context.Context, req interface{}) (interface{}, error) {
				authorizedForResult, authorizedForErr = authorization.AuthorizedFor(ctx, authorization.Resource{})
				return origResp, origErr
			}
			resp, err := interceptor(origContext, origReq, &grpc.UnaryServerInfo{FullMethod: test.method}, next)
			assert.Equal(t, test.expectErr, err)
			assert.Equal(t, test.expectResp, resp)
			assert.Equal(t, test.expectResult, authorizedForResult)
			assert.NoError(t, authorizedForErr)
		})
	}
}

func TestMetadataMethodAuthorizationHandler(t *testing.T) {
	authBuilder, _ := authorization.NewBuilder(config.AgentAuth{SecretToken: "abc123"})
	authHandler := authBuilder.ForPrivilege(authorization.PrivilegeEventWrite.Action)
	methodHandler := interceptors.MetadataMethodAuthorizationHandler(authHandler)

	ctx := context.Background()
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer abc123"))
	auth := methodHandler(ctx, nil)
	result, err := auth.AuthorizedFor(ctx, authorization.Resource{})
	require.NoError(t, err)
	assert.Equal(t, authorization.Result{Authorized: true}, result)
}

type authorizationFunc func(context.Context, authorization.Resource) (authorization.Result, error)

func (f authorizationFunc) AuthorizedFor(ctx context.Context, resource authorization.Resource) (authorization.Result, error) {
	return f(ctx, resource)
}
