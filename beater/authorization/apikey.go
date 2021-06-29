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
	"net/http"
	"time"

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

type apikeyBuilder struct {
	esClient        es.Client
	cache           *privilegesCache
	anyOfPrivileges []es.PrivilegeAction
}

type apikeyAuth struct {
	*apikeyBuilder
	// key is base64(id:apiKey)
	key string
}

func newApikeyBuilder(client es.Client, cache *privilegesCache, anyOfPrivileges []es.PrivilegeAction) *apikeyBuilder {
	return &apikeyBuilder{client, cache, anyOfPrivileges}
}

func (a *apikeyBuilder) forKey(key string) *apikeyAuth {
	return &apikeyAuth{a, key}
}

// AuthorizedFor checks if the configured api key is authorized.
//
// An API Key is considered to be authorized when the API Key has the configured privileges
// for the requested resource. Permissions are fetched from Elasticsearch and then cached in
// a global cache.
func (a *apikeyAuth) AuthorizedFor(ctx context.Context, _ Resource) (Result, error) {
	// TODO if resource is non-zero, map to different application resources in the privilege queries.
	//
	// For now, having any valid "apm" application API Key grants access to any agent and service.
	// In the future, it should be possible to have API Keys that can be restricted to a set of agent
	// and service names.
	esResource := ResourceInternal

	privileges := a.cache.get(id(a.key, esResource))
	if privileges != nil {
		return Result{Authorized: a.allowed(privileges)}, nil
	}

	if a.cache.isFull() {
		return Result{}, errors.New("api_key limit reached, " +
			"check your logs for failed authorization attempts " +
			"or consider increasing config option `apm-server.api_key.limit`")
	}

	privileges, err := a.queryES(ctx, esResource)
	if err != nil {
		return Result{}, err
	}
	a.cache.add(id(a.key, esResource), privileges)
	return Result{Authorized: a.allowed(privileges)}, nil
}

func (a *apikeyAuth) allowed(permissions es.Permissions) bool {
	var allowed bool
	for _, privilege := range a.anyOfPrivileges {
		if privilege == ActionAny {
			for _, value := range permissions {
				allowed = allowed || value
			}
		}
		allowed = allowed || permissions[privilege]
	}
	return allowed
}

func (a *apikeyAuth) queryES(ctx context.Context, resource es.Resource) (es.Permissions, error) {
	request := es.HasPrivilegesRequest{
		Applications: []es.Application{{
			Name: Application,
			// it is important to query all privilege actions because they are cached by api key+resources
			// querying a.anyOfPrivileges would result in an incomplete cache entry
			Privileges: ActionsAll(),
			Resources:  []es.Resource{resource},
		}},
	}
	info, err := es.HasPrivileges(ctx, a.esClient, request, a.key)
	if err != nil {
		var eserr *es.Error
		if errors.As(err, &eserr) && eserr.StatusCode == http.StatusUnauthorized {
			return es.Permissions{}, nil
		}
		return nil, err
	}
	if permissions, ok := info.Application[Application][resource]; ok {
		return permissions, nil
	}
	return es.Permissions{}, nil
}

func id(apiKey string, resource es.Resource) string {
	return apiKey + "_" + string(resource)
}
