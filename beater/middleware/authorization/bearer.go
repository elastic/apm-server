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
	"crypto/subtle"
)

// Bearer implements the request.Authorization interface. It compares a required and a provided token.
type Bearer struct {
	authorized bool
	enabled    bool
}

// NewBearer creates a Bearer instance based on the required and the provided token.
func NewBearer(requiredToken, requestToken string) *Bearer {
	return &Bearer{
		authorized: subtle.ConstantTimeCompare([]byte(requiredToken), []byte(requestToken)) == 1,
		enabled:    requiredToken != ""}
}

// AuthorizedFor will return true if the required and provided token are the same.
func (b *Bearer) AuthorizedFor(_, _ string) (bool, error) {
	return b.authorized, nil
}

// IsAuthorizationConfigured will return true if a non-empty token is required.
func (b *Bearer) IsAuthorizationConfigured() bool {
	return b.enabled
}
