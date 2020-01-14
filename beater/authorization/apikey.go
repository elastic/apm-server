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
	"time"

	"github.com/pkg/errors"

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
	ResourceAny      = es.Resource("*")
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

// IsAuthorizationConfigured will return true if a non-empty token is required.
func (a *apikeyAuth) IsAuthorizationConfigured() bool {
	return true
}

// AuthorizedFor checks if the configured api key is authorized.
// An api key is considered to be authorized when the api key has the configured privileges for the requested resource.
// Permissions are fetched from Elasticsearch and then cached in a global cache.
func (a *apikeyAuth) AuthorizedFor(resource es.Resource) (bool, error) {
	privileges := a.cache.get(id(a.key, resource))
	if privileges != nil {
		return a.allowed(privileges), nil
	}

	if a.cache.isFull() {
		return false, errors.New("api_key limit reached, " +
			"check your logs for failed authorization attempts " +
			"or consider increasing config option `apm-server.api_key.limit`")
	}

	privileges, err := a.queryES(resource)
	if err != nil {
		return false, err
	}
	a.cache.add(id(a.key, resource), privileges)
	return a.allowed(privileges), nil
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

func (a *apikeyAuth) queryES(resource es.Resource) (es.Permissions, error) {
	request := es.HasPrivilegesRequest{
		Applications: []es.Application{
			{
				Name: Application,
				// it is important to query all privilege actions because they are cached by api key+resources
				// querying a.anyOfPrivileges would result in an incomplete cache entry
				Privileges: ActionsAll(),
				Resources:  []es.Resource{resource},
			},
		},
	}
	info, err := es.HasPrivileges(a.esClient, request, a.key)
	if err != nil {
		return nil, err
	}
	if resources, ok := info.Application[Application]; ok {
		if permissions, ok := resources[resource]; ok {
			return permissions, nil
		}
	}
	return es.Permissions{}, nil
}

func id(apiKey string, resource es.Resource) string {
	return apiKey + "_" + string(resource)
}
