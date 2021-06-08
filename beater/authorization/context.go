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

package authorization

import (
	"context"
	"errors"
)

// ErrNoAuthorization is returned from AuthorizedFor when the context does not
// contain an Authorization.
var ErrNoAuthorization = errors.New("no authorization.Authorization in context")

type authorizationKey struct{}

// ContextWithAuthorization returns a copy of parent associated with auth.
func ContextWithAuthorization(parent context.Context, auth Authorization) context.Context {
	return context.WithValue(parent, authorizationKey{}, auth)
}

// authorizationFromContext returns the Authorization stored in ctx, if any,
// and a boolean indicating whether there one was found. The boolean is false
// if and only if the authorization is nil.
func authorizationFromContext(ctx context.Context) (Authorization, bool) {
	auth, ok := ctx.Value(authorizationKey{}).(Authorization)
	return auth, ok
}

// AuthorizedFor is a shortcut for obtaining a Handler from ctx using
// HandlerFromContext, and calling its AuthorizedFor method. AuthorizedFor
// returns an error if ctx does not contain a Handler.
func AuthorizedFor(ctx context.Context, resource Resource) (Result, error) {
	auth, ok := authorizationFromContext(ctx)
	if !ok {
		return Result{}, ErrNoAuthorization
	}
	return auth.AuthorizedFor(ctx, resource)
}
