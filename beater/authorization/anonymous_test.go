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

package authorization_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
)

func TestAnonymousAuth(t *testing.T) {
	for name, test := range map[string]struct {
		allowAgent   []string
		allowService []string
		resource     authorization.Resource
		expectResult authorization.Result
	}{
		"allow_any": {
			allowAgent:   nil,
			allowService: nil,
			resource:     authorization.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectResult: authorization.Result{Authorized: true, Anonymous: true},
		},
		"allow_agent": {
			allowAgent:   []string{"iOS/swift"},
			allowService: nil,
			resource:     authorization.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectResult: authorization.Result{Authorized: true, Anonymous: true},
		},
		"deny_agent": {
			allowAgent:   []string{"rum-js"},
			allowService: nil,
			resource:     authorization.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectResult: authorization.Result{Authorized: false, Anonymous: true, Reason: `agent "iOS/swift" not allowed`},
		},
		"allow_service": {
			allowService: []string{"opbeans-ios"},
			resource:     authorization.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectResult: authorization.Result{Authorized: true, Anonymous: true},
		},
		"deny_service": {
			allowService: []string{"opbeans-rum"},
			resource:     authorization.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectResult: authorization.Result{Authorized: false, Anonymous: true, Reason: `service "opbeans-ios" not allowed`},
		},
		"allow_agent_unspecified": {
			allowAgent:   []string{"iOS/swift"},
			resource:     authorization.Resource{ServiceName: "opbeans-ios"}, // AgentName not specified
			expectResult: authorization.Result{Authorized: true, Anonymous: true},
		},
		"allow_service_unspecified": {
			allowService: []string{"opbeans-ios"},
			resource:     authorization.Resource{AgentName: "iOS/swift"}, // ServiceName not specified
			expectResult: authorization.Result{Authorized: true, Anonymous: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			auth := getAnonymousAuth(t, config.AnonymousAgentAuth{
				Enabled:      true,
				AllowAgent:   test.allowAgent,
				AllowService: test.allowService,
			})
			result, err := auth.AuthorizedFor(context.Background(), test.resource)
			require.NoError(t, err)
			assert.Equal(t, test.expectResult, result)
		})
	}
}

func getAnonymousAuth(t testing.TB, cfg config.AnonymousAgentAuth) authorization.Authorization {
	builder, err := authorization.NewBuilder(config.AgentAuth{Anonymous: cfg})
	require.NoError(t, err)
	return builder.ForAnyOfPrivileges().AuthorizationFor("", "")
}
