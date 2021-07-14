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

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
)

func TestAnonymousAuthorizer(t *testing.T) {
	for name, test := range map[string]struct {
		allowAgent   []string
		allowService []string
		action       auth.Action
		resource     auth.Resource
		expectErr    error
	}{
		"deny_sourcemap_upload": {
			allowAgent:   nil,
			allowService: nil,
			action:       auth.ActionSourcemapUpload,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectErr:    fmt.Errorf(`%w: anonymous access not permitted for sourcemap uploads`, auth.ErrUnauthorized),
		},
		"deny_unknown_action": {
			allowAgent:   nil,
			allowService: nil,
			action:       "discombobulate",
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectErr:    errors.New(`unknown action "discombobulate"`),
		},
		"allow_any_agent_config": {
			allowAgent:   nil,
			allowService: nil,
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
		},
		"allow_any_ingest": {
			allowAgent:   nil,
			allowService: nil,
			action:       auth.ActionEventIngest,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
		},
		"allow_agent": {
			allowAgent:   []string{"iOS/swift"},
			allowService: nil,
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
		},
		"deny_agent": {
			allowAgent:   []string{"rum-js"},
			allowService: nil,
			action:       auth.ActionEventIngest,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectErr:    fmt.Errorf(`%w: anonymous access not permitted for agent "iOS/swift"`, auth.ErrUnauthorized),
		},
		"allow_service": {
			allowService: []string{"opbeans-ios"},
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
		},
		"deny_service": {
			allowService: []string{"opbeans-rum"},
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift", ServiceName: "opbeans-ios"},
			expectErr:    fmt.Errorf(`%w: anonymous access not permitted for service "opbeans-ios"`, auth.ErrUnauthorized),
		},
		"allow_agent_config_agent_unspecified": {
			allowAgent: []string{"iOS/swift"},
			action:     auth.ActionAgentConfig,
			resource:   auth.Resource{ServiceName: "opbeans-ios"}, // AgentName not checked for agent config
		},
		"deny_agent_config_service_unspecified": {
			allowService: []string{"opbeans-ios"},
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift"},
			expectErr:    fmt.Errorf(`%w: anonymous access not permitted for service ""`, auth.ErrUnauthorized),
		},
		"deny_event_ingest_agent_unspecified": {
			allowAgent: []string{"iOS/swift"},
			action:     auth.ActionEventIngest,
			resource:   auth.Resource{ServiceName: "opbeans-ios"},
			expectErr:  fmt.Errorf(`%w: anonymous access not permitted for agent ""`, auth.ErrUnauthorized),
		},
		"deny_event_ingest_service_unspecified": {
			allowService: []string{"opbeans-ios"},
			action:       auth.ActionAgentConfig,
			resource:     auth.Resource{AgentName: "iOS/swift"},
			expectErr:    fmt.Errorf(`%w: anonymous access not permitted for service ""`, auth.ErrUnauthorized),
		},
	} {
		t.Run(name, func(t *testing.T) {
			authorizer := getAnonymousAuthorizer(t, config.AnonymousAgentAuth{
				Enabled:      true,
				AllowAgent:   test.allowAgent,
				AllowService: test.allowService,
			})
			err := authorizer.Authorize(context.Background(), test.action, test.resource)
			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func getAnonymousAuthorizer(t testing.TB, cfg config.AnonymousAgentAuth) auth.Authorizer {
	authenticator, err := auth.NewAuthenticator(config.AgentAuth{
		SecretToken: "whatever", // required to enable anonymous auth
		Anonymous:   cfg,
	})
	require.NoError(t, err)
	_, authorizer, err := authenticator.Authenticate(context.Background(), "", "")
	require.NoError(t, err)
	return authorizer
}
