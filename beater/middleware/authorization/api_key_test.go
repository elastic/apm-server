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
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/beatertest"
)

func TestAPIKey_AuthorizationRequired(t *testing.T) {
	handler := NewAPIKey(&elasticsearch.Client{}, &APIKeyCache{}, "")
	assert.True(t, handler.IsAuthorizationConfigured())
}

func TestAPIKey_AuthorizedFor(t *testing.T) {
	t.Run("unsupported privilege", func(t *testing.T) {
		handler := APIKey{}
		allowed, err := handler.AuthorizedFor("", "")
		assert.False(t, allowed)
		require.Error(t, err)
	})

	t.Run("cache full", func(t *testing.T) {
		handler := &APIKey{
			globalCache: NewAPIKeyCache(time.Second, 1, nil),
			localCache:  &localCache{store: map[string]privileges{}}}
		handler.globalCache.add("abc", "xyz", nil)
		authorized, err := handler.AuthorizedFor("foo", PrivilegeIntake)
		assert.Error(t, err)
		assert.False(t, authorized)
	})

	t.Run("fetch local", func(t *testing.T) {
		applicationToTest := "service-abc"
		privilegeToTest := PrivilegeAgentConfig

		for name, tc := range map[string]struct {
			store       map[string]privileges
			application string

			authorized bool
		}{
			"backendApp_privilegeFull": {
				store:       map[string]privileges{appBackend: {privilegeFull: true}},
				application: appBackend,
				authorized:  true},
			"backendApp_privilegeToTest": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: true}},
				application: appBackend,
				authorized:  true},
			"backendApp_stay_local_if_exists": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: false}},
				application: appBackend,
				authorized:  false},
			"app_privilegeFull_fromBackendApp": {
				store:       map[string]privileges{appBackend: {privilegeFull: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilege_fromBackendApp": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilegeFull": {
				store:       map[string]privileges{applicationToTest: {privilegeFull: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilege": {
				store:       map[string]privileges{applicationToTest: {privilegeFull: false, privilegeToTest: true}},
				application: applicationToTest,
				authorized:  true},
			"app_stay_local_if_exists": {
				store:       map[string]privileges{applicationToTest: {privilegeFull: false, privilegeToTest: false}},
				application: applicationToTest,
				authorized:  false},
		} {
			t.Run(name, func(t *testing.T) {
				// setup
				handler := &APIKey{localCache: &localCache{store: tc.store}}

				// test logic
				authorized, err := handler.AuthorizedFor(tc.application, privilegeToTest)
				require.NoError(t, err)
				assert.Equal(t, tc.authorized, authorized)
			})
		}
	})

	t.Run("fetch global", func(t *testing.T) {
		applicationToTest := "service-abc"
		privilegeToTest := PrivilegeAgentConfig

		for name, tc := range map[string]struct {
			store       map[string]privileges
			application string

			authorized bool
		}{
			"backendApp_privilegeFull": {
				store:       map[string]privileges{appBackend: {privilegeFull: true}},
				application: appBackend,
				authorized:  true},
			"backendApp_privilegeToTest": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: true}},
				application: appBackend,
				authorized:  true},
			"backendApp_stay_in_cache_if_exists": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: false}},
				application: appBackend,
				authorized:  false},
			"app_privilegeFull_fromBackendApp": {
				store:       map[string]privileges{appBackend: {privilegeFull: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilege_fromBackendApp": {
				store:       map[string]privileges{appBackend: {privilegeFull: false, privilegeToTest: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilegeFull": {
				store: map[string]privileges{
					appBackend:        {privilegeFull: false, privilegeToTest: false},
					applicationToTest: {privilegeFull: true}},
				application: applicationToTest,
				authorized:  true},
			"app_privilege": {
				store:       map[string]privileges{applicationToTest: {privilegeFull: false, privilegeToTest: true}},
				application: applicationToTest,
				authorized:  true},
			"app_stay_in_cache_if_exists": {
				store:       map[string]privileges{applicationToTest: {privilegeFull: false, privilegeToTest: false}},
				application: applicationToTest,
				authorized:  false},
		} {
			t.Run(name, func(t *testing.T) {
				// setup
				token := "abc"
				handler := NewAPIKey(nil, NewAPIKeyCache(time.Second, 100, nil), token)
				for application, privileges := range tc.store {
					handler.globalCache.add(token, application, privileges)
				}

				// test privilege fetched from global cache
				authorized, err := handler.AuthorizedFor(tc.application, privilegeToTest)
				require.NoError(t, err)
				assert.Equal(t, tc.authorized, authorized)

				// test privilege set in local cache
				for application, privileges := range tc.store {
					localPrivileges := handler.localCache.get(token, application)
					if application == tc.application {
						// privileges for app that had been found and satisfied query are copied to local cache
						assert.Equal(t, privileges, localPrivileges)
					} else {
						// other privileges are not copied
						assert.Nil(t, localPrivileges)
					}
				}
			})
		}
	})

	t.Run("fetch elasticsearch", func(t *testing.T) {
		applicationToTest := "service-abc"
		privilegeToTest := PrivilegeAgentConfig

		//privilegesFull := privileges{privilegeFull: true}
		//privilegesIntake := privileges{privilegeFull: false, PrivilegeIntake: true,
		//	PrivilegeAgentConfig: false, PrivilegeAccess: true, PrivilegeSourcemap: false}
		privilegesAgentConfig := privileges{privilegeFull: false, PrivilegeIntake: false,
			PrivilegeAgentConfig: true, PrivilegeAccess: false, PrivilegeSourcemap: false}

		for name, tc := range map[string]struct {
			application string
			privileges  privileges
			authorized  bool
		}{
			//"applicationBackend": {
			//	application: appBackend, privileges: privilegesFull, authorized: true},
			//"application": {
			//	application: applicationToTest, privileges: privilegesIntake},
			//"applicationBackendAuthorized": {
			//	application: appBackend, privileges: privilegesAgentConfig, authorized: true},
			"applicationAuthorized": {
				application: applicationToTest, privileges: privilegesAgentConfig, authorized: true},
		} {
			t.Run(name, func(t *testing.T) {
				// setup
				transport := &beatertest.MockESTransport{
					RoundTripFn: func(req *http.Request) (*http.Response, error) {
						// read request body that is sent to Elasticsearch and return testcases privileges
						// for all requested apps
						decoder := json.NewDecoder(req.Body)
						var body map[string][]map[string]interface{}
						require.NoError(t, decoder.Decode(&body))
						esPrivileges := map[string]map[string]privileges{}
						for k, entries := range body {
							if k != "application" {
								continue
							}
							for _, entry := range entries {
								for key, val := range entry {
									if key == "application" {
										esPrivileges[val.(string)] = map[string]privileges{resources: tc.privileges}
									}
								}
							}
						}
						esResponse, err := mockESResponse(esPrivileges)
						require.NoError(t, err)
						return &http.Response{StatusCode: http.StatusOK, Body: esResponse}, nil
					},
				}
				client, err := beatertest.MockESClient(transport)
				require.NoError(t, err)
				globalCache := NewAPIKeyCache(time.Hour, 100, client)
				token := "1234"
				handler := NewAPIKey(client, globalCache, token)

				// fetch from Elasticsearch
				authorized, err := handler.AuthorizedFor(tc.application, privilegeToTest)
				require.NoError(t, err)
				assert.Equal(t, 1, transport.Executed)
				assert.Equal(t, tc.authorized, authorized)

				// results added to global cache
				assert.Equal(t, tc.privileges, handler.globalCache.get(token, tc.application))

				// results added to local cache
				assert.Equal(t, tc.privileges, handler.localCache.get(token, tc.application))

				// no call to ES on second request
				authorized, err = handler.AuthorizedFor(tc.application, privilegeToTest)
				require.NoError(t, err)
				assert.Equal(t, 1, transport.Executed)
				assert.Equal(t, tc.authorized, authorized)
			})
		}

		t.Run("unauthorized", func(t *testing.T) {
			// setup
			client, transport, err := beatertest.MockESDefaultClient(http.StatusUnauthorized, nil)
			require.NoError(t, err)
			globalCache := NewAPIKeyCache(time.Hour, 100, client)
			token := "1234"
			handler := NewAPIKey(client, globalCache, token)

			// fetch from Elasticsearch
			authorized, err := handler.AuthorizedFor(applicationToTest, privilegeToTest)
			require.NoError(t, err)
			assert.Equal(t, 1, transport.Executed)
			assert.False(t, authorized)

			// results added to cache, no additional call to ES
			assert.NotNil(t, handler.globalCache.get(token, applicationToTest))
			assert.NotNil(t, handler.localCache.get(token, applicationToTest))

			authorized, err = handler.AuthorizedFor(applicationToTest, privilegeToTest)
			require.NoError(t, err)
			assert.Equal(t, 1, transport.Executed)
			assert.False(t, authorized)
		})

		t.Run("error response", func(t *testing.T) {
			// setup
			client, transport, err := beatertest.MockESDefaultClient(http.StatusOK, nil)
			require.NoError(t, err)
			globalCache := NewAPIKeyCache(time.Hour, 100, client)
			token := "1234"
			handler := NewAPIKey(client, globalCache, token)

			// fetch from Elasticsearch
			authorized, err := handler.AuthorizedFor(applicationToTest, privilegeToTest)
			require.Error(t, err)
			assert.Equal(t, 1, transport.Executed)
			assert.False(t, authorized)

			// results added to cache, no additional call to ES
			authorized, err = handler.AuthorizedFor(applicationToTest, privilegeToTest)
			require.Error(t, err)
			assert.Equal(t, 2, transport.Executed)
			assert.False(t, authorized)
		})

	})

}

func TestAPIKeyCache(t *testing.T) {
	t.Run("addAndGet", func(t *testing.T) {
		c := NewAPIKeyCache(time.Hour, 3, nil)
		assert.False(t, c.full())

		c.add("foo", "service-abc", privileges{"a": true, "b": false})
		require.False(t, c.full())

		c.add("foo", "service-abc", privileges{"a": false, "c": true})
		c.add("foo", "service-xyz", privileges{"a": true, "c": false})
		c.add("bar", "service-abc", privileges{"a": false, "b": true})
		require.Equal(t, 3, c.store.ItemCount())
		assert.True(t, c.full())
		assert.Equal(t, privileges{"a": false, "c": true}, c.get("foo", "service-abc"))
		assert.Equal(t, privileges{"a": true, "c": false}, c.get("foo", "service-xyz"))
		assert.Equal(t, privileges{"a": false, "b": true}, c.get("bar", "service-abc"))
		assert.Nil(t, c.get("xyz", "abc"))
	})

	t.Run("onEvict", func(t *testing.T) {
		token, application := "foo bar", "abc"

		setupCache := func(size int, statusCode int, esErr error) (*APIKeyCache, *beatertest.MockESTransport) {
			esPrivileges := map[string]map[string]privileges{application: {resources: map[string]bool{privilegeFull: true}}}
			esResponse, err := mockESResponse(esPrivileges)
			require.NoError(t, err)
			client, transport, err := beatertest.MockESDefaultClient(statusCode, esResponse)
			require.NoError(t, err)
			return NewAPIKeyCache(time.Minute, size, client), transport
		}

		t.Run("not full", func(t *testing.T) {
			c, transport := setupCache(5, http.StatusOK, nil)
			c.add(token, application, privileges{privilegeFull: true})
			c.store.Delete(c.id(token, application)) //calls evict function

			require.Equal(t, 0, transport.Executed)
			assert.Equal(t, 0, c.store.ItemCount())
		})

		t.Run("no privileges", func(t *testing.T) {
			c, transport := setupCache(1, http.StatusOK, nil)
			c.add(token, application, nil)
			c.store.Delete(c.id(token, application)) //calls evict function

			require.Equal(t, 0, transport.Executed)
			assert.Equal(t, 0, c.store.ItemCount())
		})

		t.Run("no authorized privileges", func(t *testing.T) {
			c, transport := setupCache(1, http.StatusOK, nil)
			c.add(token, application, privileges{privilegeFull: false})
			c.store.Delete(c.id(token, application)) //calls evict function

			require.Equal(t, 0, transport.Executed)
			assert.Equal(t, 0, c.store.ItemCount())
		})

		t.Run("ES successful refetch", func(t *testing.T) {
			c, transport := setupCache(1, http.StatusOK, nil)
			c.add(token, application, privileges{privilegeFull: false, PrivilegeAccess: true})
			c.store.Delete(c.id(token, application)) //calls evict function

			require.Equal(t, 1, transport.Executed)
			require.Equal(t, 1, c.store.ItemCount())
			privilegesAfterEviction := c.get(token, application)
			require.NotNil(t, privilegesAfterEviction)
			assert.True(t, privilegesAfterEviction[privilegeFull])
		})

		t.Run("ES refetch nil", func(t *testing.T) {
			c, transport := setupCache(1, http.StatusUnauthorized, nil)
			c.add(token, application, privileges{privilegeFull: false, PrivilegeAccess: true})

			c.store.Delete(c.id(token, application)) //calls evict function

			assert.Equal(t, 1, transport.Executed)
			assert.Equal(t, 0, c.store.ItemCount())
		})

		t.Run("ES refetch with error", func(t *testing.T) {
			c, transport := setupCache(1, http.StatusUnauthorized, errors.New(""))
			c.add(token, application, privileges{privilegeFull: false, PrivilegeAccess: true})
			c.store.Delete(c.id(token, application)) //calls evict function

			require.Equal(t, 1, transport.Executed)
			assert.Equal(t, 0, c.store.ItemCount())
		})
	})

	t.Run("id", func(t *testing.T) {
		for _, tc := range []struct {
			a, b string
		}{
			{a: "", b: ""},
			{a: "1", b: ""},
			{a: "", b: "2"},
			{a: "a s d", b: "x-23"},
		} {
			c := &APIKeyCache{}
			a, b := c.splitID(c.id(tc.a, tc.b))
			assert.Equal(t, tc.a, a)
			assert.Equal(t, tc.b, b)
		}
	})
}

func mockESResponse(esPrivileges map[string]map[string]privileges) (io.ReadCloser, error) {
	resp, err := json.Marshal(hasPrivilegesResponse{HasAllRequested: true, Applications: esPrivileges})
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(resp)), nil
}
