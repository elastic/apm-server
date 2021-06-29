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
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/ratelimit"
)

// AnonymousRateLimit returns a grpc.UnaryServerInterceptor that adds a rate limiter
// to the context of anonymous requests. RateLimit must be wrapped by the ClientMetadata
// and Authorization interceptor, as it requires the client's IP address and authorization.
func AnonymousRateLimit(store *ratelimit.Store) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		authResult, err := authorization.AuthorizedFor(ctx, authorization.Resource{})
		if err != nil {
			return nil, err
		}
		if authResult.Anonymous {
			clientMetadata, ok := ClientMetadataFromContext(ctx)
			if !ok {
				return nil, errors.New("client metadata not found in context")
			}
			limiter := store.ForIP(clientMetadata.SourceIP)
			if !limiter.Allow() {
				return nil, status.Error(
					codes.ResourceExhausted,
					ratelimit.ErrRateLimitExceeded.Error(),
				)
			}
			ctx = ratelimit.ContextWithLimiter(ctx, limiter)
		}
		result, err := handler(ctx, req)
		if errors.Is(err, ratelimit.ErrRateLimitExceeded) {
			err = status.Error(codes.ResourceExhausted, err.Error())
		}
		return result, err
	}
}
