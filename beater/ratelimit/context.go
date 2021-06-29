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

package ratelimit

import (
	"context"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// ErrRateLimitExceeded is returned when the rate limit is exceeded.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type rateLimiterKey struct{}

// FromContext returns a rate.Limiter if one is contained in ctx,
// and a bool indicating whether one was found.
func FromContext(ctx context.Context) (*rate.Limiter, bool) {
	limiter, ok := ctx.Value(rateLimiterKey{}).(*rate.Limiter)
	return limiter, ok
}

// ContextWithLimiter returns a copy of parent associated with limiter.
func ContextWithLimiter(parent context.Context, limiter *rate.Limiter) context.Context {
	return context.WithValue(parent, rateLimiterKey{}, limiter)
}
