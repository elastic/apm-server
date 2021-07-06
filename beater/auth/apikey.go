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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"

	es "github.com/elastic/apm-server/elasticsearch"
)

const cleanupInterval = 60 * time.Second

const (
	// Application is a constant mapped to the "application" field for the Elasticsearch security API
	// This identifies privileges and keys created for APM
	Application es.AppName = "apm"

	// ResourceInternal is only valid for first authorization of a request.
	// The API Key needs to grant privileges to additional resources for successful processing of requests.
	ResourceInternal = es.Resource("-")
)

var (
	// PrivilegeAgentConfigRead identifies the Elasticsearch API Key privilege
	// required for authorizing agent config queries.
	PrivilegeAgentConfigRead = es.NewPrivilege("agentConfig", "config_agent:read")

	// PrivilegeEventWrite identifies the Elasticsearch API Key privilege required
	// for authorizing event ingestion.
	PrivilegeEventWrite = es.NewPrivilege("event", "event:write")

	// PrivilegeSourcemapWrite identifies the Elasticsearch API Key privilege
	// required for authorizing source map uploads.
	PrivilegeSourcemapWrite = es.NewPrivilege("sourcemap", "sourcemap:write")
)

// AllPrivilegeActions returns all Elasticsearch privilege actions used by APM Server.
func AllPrivilegeActions() []es.PrivilegeAction {
	return []es.PrivilegeAction{
		PrivilegeAgentConfigRead.Action,
		PrivilegeEventWrite.Action,
		PrivilegeSourcemapWrite.Action,
	}
}

type apikeyAuth struct {
	esClient es.Client
	cache    *privilegesCache
}

type apikeyAuthorizer struct {
	permissions es.Permissions
}

func newApikeyAuth(client es.Client, cache *privilegesCache) *apikeyAuth {
	return &apikeyAuth{client, cache}
}

func (a *apikeyAuth) authenticate(ctx context.Context, credentials string) (*APIKeyAuthenticationDetails, *apikeyAuthorizer, error) {
	decoded, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrAuthFailed, err)
	}
	colon := bytes.IndexByte(decoded, ':')
	if colon == -1 {
		return nil, nil, fmt.Errorf("%w: improperly formatted ApiKey credentials: expected base64(ID:APIKey)", ErrAuthFailed)
	}
	id := string(decoded[:colon])

	// Check that the user has any privileges for the internal resource.
	response, err := a.hasPrivileges(ctx, id, credentials, ResourceInternal)
	if err != nil {
		return nil, nil, err
	}
	permissions := response.Application[Application][ResourceInternal]
	haveAny := false
	for _, havePermission := range permissions {
		if havePermission {
			haveAny = true
			break
		}
	}
	if !haveAny {
		return nil, nil, ErrAuthFailed
	}
	details := &APIKeyAuthenticationDetails{ID: id, Username: response.Username}
	return details, &apikeyAuthorizer{permissions}, nil
}

func (a *apikeyAuth) hasPrivileges(ctx context.Context, id, credentials string, resource es.Resource) (*es.HasPrivilegesResponse, error) {
	cacheKey := id + "_" + string(resource)
	if response, ok := a.cache.get(cacheKey); ok {
		if response == nil {
			return nil, ErrAuthFailed
		}
		return response, nil
	}

	if a.cache.isFull() {
		return nil, errors.New(
			"api_key limit reached, check your logs for failed authorization attempts " +
				"or consider increasing config option `apm-server.api_key.limit`",
		)
	}

	request := es.HasPrivilegesRequest{
		Applications: []es.Application{{
			Name: Application,
			// it is important to query all privilege actions because they are cached by api key+resources
			// querying a.anyOfPrivileges would result in an incomplete cache entry
			Privileges: AllPrivilegeActions(),
			Resources:  []es.Resource{resource},
		}},
	}
	info, err := es.HasPrivileges(ctx, a.esClient, request, credentials)
	if err != nil {
		var eserr *es.Error
		if errors.As(err, &eserr) && eserr.StatusCode == http.StatusUnauthorized {
			// Cache authorization failures to avoid hitting Elasticsearch every time.
			a.cache.add(cacheKey, nil)
			return nil, ErrAuthFailed
		}
		return nil, err
	}
	a.cache.add(cacheKey, &info)
	return &info, nil
}

// Authorize checks if the configured API Key is authorized for the given action and resource.
//
// An API Key is considered to be authorized when the API Key has the configured privileges
// for the requested resource. Permissions are fetched from Elasticsearch and then cached in
// a global cache.
func (a *apikeyAuthorizer) Authorize(ctx context.Context, action Action, _ Resource) error {
	// TODO if resource is non-zero, map to different application resources in the privilege queries.
	//
	// For now, having any valid "apm" application API Key grants access to any agent and service.
	// In the future, it should be possible to have API Keys that can be restricted to a set of agent
	// and service names.
	var apikeyPrivilegeAction es.PrivilegeAction
	switch action {
	case ActionAgentConfig:
		apikeyPrivilegeAction = PrivilegeAgentConfigRead.Action
	case ActionEventIngest:
		apikeyPrivilegeAction = PrivilegeEventWrite.Action
	case ActionSourcemapUpload:
		apikeyPrivilegeAction = PrivilegeSourcemapWrite.Action
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	if a.permissions[apikeyPrivilegeAction] {
		return nil
	}
	return fmt.Errorf("%w: API Key not permitted action %q", ErrUnauthorized, apikeyPrivilegeAction)
}

type privilegesCache struct {
	cache *cache.Cache
	size  int
}

func newPrivilegesCache(expiration time.Duration, size int) *privilegesCache {
	return &privilegesCache{cache: cache.New(expiration, cleanupInterval), size: size}
}

func (c *privilegesCache) isFull() bool {
	return c.cache.ItemCount() >= c.size
}

func (c *privilegesCache) get(id string) (*es.HasPrivilegesResponse, bool) {
	if val, exists := c.cache.Get(id); exists {
		return val.(*es.HasPrivilegesResponse), true
	}
	return nil, false
}

func (c *privilegesCache) add(id string, privileges *es.HasPrivilegesResponse) {
	c.cache.SetDefault(id, privileges)
}
