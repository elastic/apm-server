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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/elasticsearch"
)

const (
	application = "apm"
	sep         = `","`

	cleanupInterval = 60 * time.Second
)

type apikeyBuilder struct {
	esClient        elasticsearch.Client
	cache           *privilegesCache
	anyOfPrivileges []string
}

type apikeyAuth struct {
	*apikeyBuilder
	key string
}

type hasPrivilegesResponse struct {
	Applications map[string]map[string]privileges `json:"application"`
}

func newApikeyBuilder(client elasticsearch.Client, cache *privilegesCache, anyOfPrivileges []string) *apikeyBuilder {
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
// Privileges are fetched from Elasticsearch and then cached in a global cache.
func (a *apikeyAuth) AuthorizedFor(resource string) (bool, error) {
	if resource == "" {
		resource = DefaultResource
	}

	//fetch from cache
	if allowed, found := a.fromCache(resource); found {
		return allowed, nil
	}

	if a.cache.isFull() {
		return false, errors.New("api_key limit reached, " +
			"check your logs for failed authorization attempts " +
			"or consider increasing config option `apm-server.api_key.limit`")
	}

	//fetch from ES
	privileges, err := a.queryES(resource)
	if err != nil {
		return false, err
	}
	//add to cache
	a.cache.add(id(a.key, resource), privileges)

	allowed, _ := a.fromCache(resource)
	return allowed, nil
}

func (a *apikeyAuth) fromCache(resource string) (allowed bool, found bool) {
	privileges := a.cache.get(id(a.key, resource))
	if privileges == nil {
		return
	}
	found = true
	allowed = false
	for _, privilege := range a.anyOfPrivileges {
		if privilegeAllowed, ok := privileges[privilege]; ok && privilegeAllowed {
			allowed = true
			return
		}
	}
	return
}

func (a *apikeyAuth) queryES(resource string) (privileges, error) {
	query := buildQuery(PrivilegesAll, resource)
	statusCode, body, err := a.esClient.SecurityHasPrivilegesRequest(strings.NewReader(query),
		http.Header{headers.Authorization: []string{headers.APIKey + " " + a.key}})
	if err != nil {
		return nil, err
	}
	defer body.Close()
	if statusCode != http.StatusOK {
		// return nil privileges for queried apps to ensure they are cached
		return privileges{}, nil
	}

	var decodedResponse hasPrivilegesResponse
	if err := json.NewDecoder(body).Decode(&decodedResponse); err != nil {
		return nil, err
	}
	if resources, ok := decodedResponse.Applications[application]; ok {
		if privileges, ok := resources[resource]; ok {
			return privileges, nil
		}
	}
	return privileges{}, nil
}

func buildQuery(privileges []string, resource string) string {
	var b strings.Builder
	b.WriteString(`{"application":[{"application":"`)
	b.WriteString(application)
	b.WriteString(`","privileges":["`)
	b.WriteString(strings.Join(privileges, sep))
	b.WriteString(`"],"resources":"`)
	b.WriteString(resource)
	b.WriteString(`"}]}`)
	return b.String()
}

func id(apiKey, resource string) string {
	return apiKey + "_" + resource
}
