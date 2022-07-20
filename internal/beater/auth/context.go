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

package auth

import (
	"context"
	"errors"
)

// ErrNoAuthorizer is returned from Authorize when the context does not contain an Authorizer.
var ErrNoAuthorizer = errors.New("no Authorizer in context")

type authorizationKey struct{}

// ContextWithAuthorizer returns a copy of parent associated with auth.
func ContextWithAuthorizer(parent context.Context, auth Authorizer) context.Context {
	return context.WithValue(parent, authorizationKey{}, auth)
}

// authorizationFromContext returns the Authorizer stored in ctx, if any, and a boolean
// indicating whether there one was found. The boolean is false if and only if the
// Authorizer is nil.
func authorizationFromContext(ctx context.Context) (Authorizer, bool) {
	auth, ok := ctx.Value(authorizationKey{}).(Authorizer)
	return auth, ok
}

// Authorize is a shortcut for obtaining an Authorizer from ctx and calling its Authorize
// method. Authorize returns ErrNoAuthorizer if ctx does not contain an Authorizer.
func Authorize(ctx context.Context, action Action, resource Resource) error {
	auth, ok := authorizationFromContext(ctx)
	if !ok {
		return ErrNoAuthorizer
	}
	return auth.Authorize(ctx, action, resource)
}
