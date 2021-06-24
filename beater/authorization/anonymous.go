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

// AuthorizedFor always returns a Result indicating the request is anonymous, and
// authorized if the resource is permitted for anonymous access.
func (a *anonymousAuth) AuthorizedFor(ctx context.Context, resource Resource) (Result, error) {
	result := Result{Authorized: true, Anonymous: true}
	switch {
	case resource.AgentName != "" && len(a.allowedAgents) > 0 && !a.allowedAgents[resource.AgentName]:
		result.Authorized = false
		result.Reason = fmt.Sprintf("agent %q not allowed", resource.AgentName)
	case resource.ServiceName != "" && len(a.allowedServices) > 0 && !a.allowedServices[resource.ServiceName]:
		result.Authorized = false
		result.Reason = fmt.Sprintf("service %q not allowed", resource.ServiceName)
	}
	return result, nil
}
