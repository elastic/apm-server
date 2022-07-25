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
	"fmt"
)

func newAnonymousAuth(allowAgent, allowService []string) *anonymousAuth {
	a := &anonymousAuth{
		allowedAgents:   make(map[string]bool),
		allowedServices: make(map[string]bool),
	}
	for _, name := range allowAgent {
		a.allowedAgents[name] = true
	}
	for _, name := range allowService {
		a.allowedServices[name] = true
	}
	return a
}

// anonymousAuth implements the Authorization interface, allowing anonymous access with
// optional restriction on agent and service name.
type anonymousAuth struct {
	allowedAgents   map[string]bool
	allowedServices map[string]bool
}

// Authorize checks if anonymous access is authorized for the given action and resource.
func (a *anonymousAuth) Authorize(ctx context.Context, action Action, resource Resource) error {
	switch action {
	case ActionAgentConfig:
		// Anonymous access to agent config should be restricted by service.
		// Agent config queries do not provide an agent name, so that is not
		// checked here. Instead, the agent config handlers will filter results
		// down to those in the allowed agent list.
		if len(a.allowedServices) != 0 && !a.allowedServices[resource.ServiceName] {
			return fmt.Errorf(
				"%w: anonymous access not permitted for service %q",
				ErrUnauthorized, resource.ServiceName,
			)
		}
		return nil
	case ActionEventIngest:
		if len(a.allowedServices) != 0 && !a.allowedServices[resource.ServiceName] {
			return fmt.Errorf(
				"%w: anonymous access not permitted for service %q",
				ErrUnauthorized, resource.ServiceName,
			)
		}
		if len(a.allowedAgents) != 0 && !a.allowedAgents[resource.AgentName] {
			return fmt.Errorf(
				"%w: anonymous access not permitted for agent %q",
				ErrUnauthorized, resource.AgentName,
			)
		}
		return nil
	case ActionSourcemapUpload:
		return fmt.Errorf("%w: anonymous access not permitted for sourcemap uploads", ErrUnauthorized)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}
