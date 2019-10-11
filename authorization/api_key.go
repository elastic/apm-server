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

const (
	PrivilegeAccess      = "action:access"
	PrivilegeSourcemap   = "action:sourcemap"
	PrivilegeIntake      = "action:intake"
	PrivilegeAgentConfig = "action:config"
	privilegeFull        = "action:full"

	appBackend = "apm-backend"

	required   = true
	resources  = "*"
	sep        = `","`
	whitespace = " "

	cleanupInterval time.Duration = 60 * time.Second
)

var (
	queryPrivileges = []string{privilegeFull, PrivilegeAccess, PrivilegeIntake, PrivilegeAgentConfig, PrivilegeSourcemap}
)

type APIKey struct {
	esClient    *elasticsearch.Client
	globalCache *APIKeyCache
	localCache  *localCache
	token       string
}

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

func NewAPIKey(
	esClient *elasticsearch.Client,
	cache *APIKeyCache,
	token string,
) *APIKey {
	return &APIKey{esClient: esClient, globalCache: cache, token: token,
		localCache: &localCache{store: map[string]privileges{}}}
}

func (*APIKey) AuthorizationRequired() bool {
	return required
}

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
	privilegesPerApp, err := fromES(a.esClient, a.token, application)
	if err != nil {
		return false, err
	}
	//add to cache
	for app, priv := range privilegesPerApp {
		a.localCache.add(app, priv)
		a.globalCache.add(a.token, app, priv)
	}

	allowed, _ := a.fromCache(application, privilege)
	return allowed, nil
}

func (a *APIKey) fromCache(application, privilege string) (allowed bool, exists bool) {
	//from local cache
	if _, allowed, exists = checkPrivileges(a.localCache, a.token, application, privilege); exists {
		return
	}
	// from global cache, ensuring to store privileges in local cache if they exist
	var privileges privileges
	privileges, allowed, exists = checkPrivileges(a.globalCache, a.token, application, privilege)
	if exists {
		a.localCache.add(application, privileges)
	}
	return
}

func fromES(client *elasticsearch.Client, token string, application string) (map[string]privileges, error) {
	qb := queryBuilder{appBackend: queryPrivileges}
	if application != appBackend {
		qb[application] = queryPrivileges
	}
	hasPrivileges := esapi.SecurityHasPrivilegesRequest{
		Body:   strings.NewReader(qb.string()),
		Header: http.Header{headers.Authorization: []string{headers.APIKey + " " + token}},
	}
	resp, err := hasPrivileges.Do(context.Background(), client)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// return nil privileges for queried apps to ensure they are cached
		privilegesPerApp := map[string]privileges{}
		for app, _ := range qb {
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

func checkPrivileges(cache cache, token, application, privilege string) (privileges privileges, allowed bool, exists bool) {
	if application != appBackend {
		//check if token has privileges for default apm application
		privileges = cache.get(token, appBackend)
		if allowed, exists = privileges.findWithFullPrivilege(privilege); allowed && exists {
			return
		}
	}
	privileges = cache.get(token, application)
	allowed, exists = privileges.findWithFullPrivilege(privilege)
	return
}

func (p privileges) findWithFullPrivilege(privilege string) (bool, bool) {
	if p == nil {
		return false, false
	}
	p1, _ := p[privilegeFull]
	p2, _ := p[privilege]
	return p1 || p2, true
}

// Global Cache - threadsafe

func NewAPIKeyCache(expiration time.Duration, size int, esClient *elasticsearch.Client) *APIKeyCache {
	c := APIKeyCache{store: gocache.New(expiration, cleanupInterval), size: size}

	// the onEvicted method ensures that privileges for valid tokens stay in cache
	// this is helpful if cache is filled with malicious tokens, as valid tokens would not get
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
		token, application := c.splitID(id)
		response, err := fromES(esClient, token, application)
		if err != nil {
			return
		}
		//add to globalCache
		for application, privileges := range response {
			if len(privileges) > 0 {
				c.add(token, application, privileges)
			}
		}
	})

	return &c
}

func (c *APIKeyCache) full() bool {
	return c.store.ItemCount()+1 >= c.size
}

func (c *APIKeyCache) get(token string, application string) privileges {
	if val, exists := c.store.Get(c.id(token, application)); exists {
		return val.(privileges)
	}
	return nil
}

func (c *APIKeyCache) add(token string, application string, privileges privileges) {
	c.store.SetDefault(c.id(token, application), privileges)
}

func (c *APIKeyCache) id(token, application string) string {
	// no part of the application name can contain whitespaces
	// according to https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-put-privileges.html#security-api-app-privileges-validation
	return token + whitespace + application
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
