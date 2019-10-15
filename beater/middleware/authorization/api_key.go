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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/utility"
)

// Privileges supported by the APIKey authorization implementation.
const (
	PrivilegeAccess      = "action:access"
	PrivilegeSourcemap   = "action:sourcemap"
	PrivilegeIntake      = "action:intake"
	PrivilegeAgentConfig = "action:config"
	privilegeFull        = "action:full"
)

const (
	appBackend = "apm-backend"
	enabled    = true

	resources  = "*"
	sep        = `","`
	whitespace = " "

	cleanupInterval = 60 * time.Second
)

var (
	queryPrivileges = []string{privilegeFull, PrivilegeAccess, PrivilegeIntake, PrivilegeAgentConfig, PrivilegeSourcemap}
)

// APIKey implements the request.Authorization interface. It uses an external Elasticsearch instance to verify
// api key privileges for applications.
type APIKey struct {
	esClient    *elasticsearch.Client
	globalCache *APIKeyCache
	localCache  *localCache
	apiKey      string
}

// APIKeyCache is a cache to store fetched API Key privileges per apiKey and application.
// The cache is passed in as a parameter when creating a new API Key authorization handler.
type APIKeyCache struct {
	store *gocache.Cache
	size  int
}

type localCache struct {
	store map[string]privileges
}

type cache interface {
	get(string, string) privileges
}

type hasPrivilegesResponse struct {
	HasAllRequested bool                             `json:"has_all_requested"`
	Applications    map[string]map[string]privileges `json:"application"`
}

type privileges map[string]bool
type queryBuilder map[string][]string

// NewAPIKey creates a new instance of an APIKey handler. An Elasticsearch instance, a global cache instance and the
// APIKey are required parameters.
func NewAPIKey(
	esClient *elasticsearch.Client,
	cache *APIKeyCache,
	apiKey string,
) *APIKey {
	return &APIKey{esClient: esClient, globalCache: cache, apiKey: apiKey,
		localCache: &localCache{store: map[string]privileges{}}}
}

// IsAuthorizationConfigured always returns true for API Key authorization
func (*APIKey) IsAuthorizationConfigured() bool {
	return enabled
}

// AuthorizedFor checks if the configured api key is authorized.
// An api key is considered to be authorized when the api key has the requested privilege or the `privilegeFull` for
// either the requested application or the generic `appBackend`.
//
// It first tries to get the information from a local cache. If not available, it fetches it from the global cache,
// if also not available the privileges are fetched from the configured Elasticsearch client.
// The information is pushed to the cache instances if necessary.
func (a *APIKey) AuthorizedFor(application string, privilege string) (bool, error) {
	//check that privilege is generally supported
	if !utility.Contains(privilege, queryPrivileges) {
		return false, errors.Errorf("privilege '%s' is unsupported", privilege)
	}

	if application == "" {
		application = appBackend
	}

	//fetch from cache
	if allowed, found := a.fromCache(application, privilege); found {
		return allowed, nil
	}

	if a.globalCache.full() {
		return false, errors.New("full cache, " +
			"check your logs for failed authorization attempts " +
			"and consider increasing `apm-server.authorization.api_key.cache.size` setting")
	}

	//fetch from ES
	privilegesPerApp, err := fromES(a.esClient, a.apiKey, application)
	if err != nil {
		return false, err
	}
	//add to cache
	for app, priv := range privilegesPerApp {
		a.localCache.add(app, priv)
		a.globalCache.add(a.apiKey, app, priv)
	}

	allowed, _ := a.fromCache(application, privilege)
	return allowed, nil
}

func (a *APIKey) fromCache(application, privilege string) (allowed bool, exists bool) {
	//from local cache
	if _, allowed, exists = checkPrivileges(a.localCache, a.apiKey, application, privilege); exists {
		return
	}
	// from global cache, ensuring to store privileges in local cache if they exist
	var privileges privileges
	privileges, allowed, exists = checkPrivileges(a.globalCache, a.apiKey, application, privilege)
	if exists {
		a.localCache.add(application, privileges)
	}
	return
}

func fromES(client *elasticsearch.Client, apiKey string, application string) (map[string]privileges, error) {
	qb := queryBuilder{appBackend: queryPrivileges}
	if application != appBackend {
		qb[application] = queryPrivileges
	}
	hasPrivileges := esapi.SecurityHasPrivilegesRequest{
		Body:   strings.NewReader(qb.string()),
		Header: http.Header{headers.Authorization: []string{headers.APIKey + " " + apiKey}},
	}
	resp, err := hasPrivileges.Do(context.Background(), client)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// return nil privileges for queried apps to ensure they are cached
		privilegesPerApp := map[string]privileges{}
		for app := range qb {
			privilegesPerApp[app] = privileges{}
		}
		return privilegesPerApp, nil
	}

	var decodedResponse hasPrivilegesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decodedResponse); err != nil {
		return nil, err
	}
	privilegesPerApp := map[string]privileges{}
	for app, resourcePrivileges := range decodedResponse.Applications {
		for resource, privileges := range resourcePrivileges {
			if resource != resources {
				continue
			}
			privilegesPerApp[app] = privileges
		}
	}

	return privilegesPerApp, nil
}

func (q queryBuilder) string() string {
	var b strings.Builder
	b.WriteString(`{"application":[`)
	var count int
	for application, privileges := range q {
		count++
		if count > 1 {
			b.WriteString(",")
		}
		b.WriteString(`{"application":"`)
		b.WriteString(application)
		b.WriteString(`","privileges":["`)
		b.WriteString(strings.Join(privileges, sep))
		b.WriteString(`"],"resources":"`)
		b.WriteString(resources)
		b.WriteString(`"}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

func checkPrivileges(cache cache, apiKey, application, privilege string) (privileges privileges, allowed bool, exists bool) {
	if application != appBackend {
		//check if apiKey has privileges for default apm application
		privileges = cache.get(apiKey, appBackend)
		if allowed, exists = privileges.findWithFullPrivilege(privilege); allowed && exists {
			return
		}
	}
	privileges = cache.get(apiKey, application)
	allowed, exists = privileges.findWithFullPrivilege(privilege)
	return
}

func (p privileges) findWithFullPrivilege(privilege string) (bool, bool) {
	if p == nil {
		return false, false
	}
	return p[privilegeFull] || p[privilege], true
}

// Global Cache - threadsafe

// NewAPIKeyCache creates an APIKeyCache instance with the given auto expiration and of given size.
// The Elasticsearch client instance is used to refetch application privileges from Elasticsearch when tokens
// are expired but had cached valid privileges.
func NewAPIKeyCache(expiration time.Duration, size int, esClient *elasticsearch.Client) *APIKeyCache {
	c := APIKeyCache{store: gocache.New(expiration, cleanupInterval), size: size}

	// the onEvicted method ensures that privileges for valid apiKey stay in cache
	// this is helpful if cache is filled with malicious apiKey, as valid apiKey would not get
	// displaced by invalid ones
	c.store.OnEvicted(func(id string, val interface{}) {
		if !c.full() {
			return
		}
		privileges, ok := val.(privileges)
		if !ok || privileges == nil {
			return
		}

		allowed := false
		for _, hasPrivilege := range privileges {
			if hasPrivilege {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}

		//fetch from ES
		apiKey, application := c.splitID(id)
		response, err := fromES(esClient, apiKey, application)
		if err != nil {
			return
		}
		//add to globalCache
		for application, privileges := range response {
			if len(privileges) > 0 {
				c.add(apiKey, application, privileges)
			}
		}
	})

	return &c
}

func (c *APIKeyCache) full() bool {
	return c.store.ItemCount()+1 >= c.size
}

func (c *APIKeyCache) get(apiKey string, application string) privileges {
	if val, exists := c.store.Get(c.id(apiKey, application)); exists {
		return val.(privileges)
	}
	return nil
}

func (c *APIKeyCache) add(apiKey string, application string, privileges privileges) {
	c.store.SetDefault(c.id(apiKey, application), privileges)
}

func (c *APIKeyCache) id(apiKey, application string) string {
	// no part of the application name can contain whitespaces
	// according to https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-put-privileges.html#security-api-app-privileges-validation
	return apiKey + whitespace + application
}

func (c *APIKeyCache) splitID(id string) (string, string) {
	delimiterIdx := strings.LastIndex(id, whitespace)
	return id[0:delimiterIdx], id[delimiterIdx+1:]
}

// Local Cache - not threadsafe
// In case multiple auth requests are necessary for one request avoid hitting the global cache every time by keeping
// and in-memory representation of the requests privileges.

func (c *localCache) get(_ string, application string) privileges {
	if privileges, exists := c.store[application]; exists {
		return privileges
	}
	return nil
}

func (c *localCache) add(application string, privileges privileges) {
	c.store[application] = privileges
}
