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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/ratelimit"
)

func TestAnonymousRateLimit(t *testing.T) {
	type test struct {
		burst     int
		anonymous bool

		expectErr   error
		expectAllow bool
	}
	for _, test := range []test{{
		burst:     0,
		anonymous: false,
		expectErr: nil,
	}, {
		burst:     0,
		anonymous: true,
		expectErr: status.Error(codes.ResourceExhausted, "rate limit exceeded"),
	}, {
		burst:       1,
		anonymous:   true,
		expectErr:   nil,
		expectAllow: false,
	}, {
		burst:       2,
		anonymous:   true,
		expectErr:   nil,
		expectAllow: true,
	}} {
		store, _ := ratelimit.NewStore(1, 1, test.burst)
		interceptor := interceptors.AnonymousRateLimit(store)
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			limiter, ok := ratelimit.FromContext(ctx)
			if test.anonymous {
				require.True(t, ok)
				assert.Equal(t, test.expectAllow, limiter.Allow())
			} else {
				require.False(t, ok)
			}
			return "response", nil
		}

		ctx := interceptors.ContextWithClientMetadata(context.Background(),
			interceptors.ClientMetadataValues{SourceIP: net.ParseIP("10.2.3.4")},
		)
		ctx = authorization.ContextWithAuthorization(ctx,
			authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
				return authorization.Result{Authorized: true, Anonymous: test.anonymous}, nil
			}),
		)
		resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{}, handler)
		if test.expectErr != nil {
			assert.Nil(t, resp)
			assert.Equal(t, test.expectErr, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, "response", resp)
		}
	}
}

func TestAnonymousRateLimitForIP(t *testing.T) {
	store, _ := ratelimit.NewStore(2, 1, 1)
	interceptor := interceptors.AnonymousRateLimit(store)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }

	requestWithIP := func(ip string) error {
		ctx := interceptors.ContextWithClientMetadata(context.Background(),
			interceptors.ClientMetadataValues{SourceIP: net.ParseIP(ip)},
		)
		ctx = authorization.ContextWithAuthorization(ctx,
			authorizationFunc(func(ctx context.Context, _ authorization.Resource) (authorization.Result, error) {
				return authorization.Result{Authorized: true, Anonymous: true}, nil
			}),
		)
		_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{}, handler)
		return err
	}
	assert.NoError(t, requestWithIP("10.1.1.1"))
	assert.Equal(t, status.Error(codes.ResourceExhausted, "rate limit exceeded"), requestWithIP("10.1.1.1"))
	assert.NoError(t, requestWithIP("10.1.1.2"))

	// ratelimit.Store size is 2: the 3rd IP reuses an existing (depleted) rate limiter.
	assert.Equal(t, status.Error(codes.ResourceExhausted, "rate limit exceeded"), requestWithIP("10.1.1.3"))
}
