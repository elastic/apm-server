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
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/elasticsearch/estest"
)

func TestApikeyBuilder(t *testing.T) {
	// in case handler does not read from cache, but from ES an error is returned
	tc := &apikeyTestcase{
		cache:     newPrivilegesCache(time.Minute, 5),
		transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}

	tc.setup(t)
	key := "myApiKey"
	handler1 := tc.builder.forKey(key)
	handler2 := tc.builder.forKey(key)

	// add existing privileges to shared cache
	privilegesValid := privileges{}
	for _, p := range PrivilegesAll {
		privilegesValid[p] = true
	}
	resource := "service-go"
	tc.cache.add(id(key, resource), privilegesValid)

	// check that cache is actually shared between apiKeyHandlers
	allowed, err := handler1.AuthorizedFor(resource)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = handler2.AuthorizedFor(resource)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestApikeyAuth_IsAuthorizationConfigured(t *testing.T) {
	tc := &apikeyTestcase{}
	tc.setup(t)
	assert.True(t, tc.builder.forKey("xyz").IsAuthorizationConfigured())
}

func TestAPIKey_AuthorizedFor(t *testing.T) {
	t.Run("cache full", func(t *testing.T) {
		tc := &apikeyTestcase{cache: newPrivilegesCache(time.Millisecond, 1)}
		tc.setup(t)
		handler := tc.builder.forKey("")

		authorized, err := handler.AuthorizedFor("data:ingest")
		assert.False(t, authorized)
		assert.NoError(t, err)

		authorized, err = handler.AuthorizedFor("apm:read")
		assert.Error(t, err)
		assert.False(t, authorized)
	})

	t.Run("from cache", func(t *testing.T) {
		// in case handler does not read from cache, but from ES an error is returned
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}
		tc.setup(t)
		key := ""
		handler := tc.builder.forKey(key)
		resourceValid := "foo"
		resourceInvalid := "bar"
		resourceMissing := "missing"

		tc.cache.add(id(key, resourceValid), privileges{tc.anyOfPrivileges[0]: true})
		tc.cache.add(id(key, resourceInvalid), privileges{tc.anyOfPrivileges[0]: false})

		valid, err := handler.AuthorizedFor(resourceValid)
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = handler.AuthorizedFor(resourceInvalid)
		require.NoError(t, err)
		assert.False(t, valid)

		valid, err = handler.AuthorizedFor(resourceMissing)
		require.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("from ES", func(t *testing.T) {
		tc := &apikeyTestcase{}
		tc.setup(t)
		handler := tc.builder.forKey("key")

		valid, err := handler.AuthorizedFor("foo")
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = handler.AuthorizedFor("bar")
		require.NoError(t, err)
		assert.False(t, valid)

		valid, err = handler.AuthorizedFor("missing")
		require.NoError(t, err)
		assert.False(t, valid)
		assert.Equal(t, 3, tc.cache.cache.ItemCount())
	})

	t.Run("error from ES", func(t *testing.T) {
		tc := &apikeyTestcase{
			transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("12a3")

		valid, err := handler.AuthorizedFor("xyz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Internal server error")
		assert.False(t, valid)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})

	t.Run("invalid status from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusNotFound, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("12a3")

		valid, err := handler.AuthorizedFor("xyz")
		require.NoError(t, err)
		assert.False(t, valid)
		assert.Equal(t, 1, tc.cache.cache.ItemCount())
	})

	t.Run("decode error from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusOK, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("123")
		valid, err := handler.AuthorizedFor("foo")
		require.Error(t, err)
		assert.False(t, valid)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})
}

type apikeyTestcase struct {
	transport       *estest.Transport
	client          elasticsearch.Client
	cache           *privilegesCache
	anyOfPrivileges []string

	builder *apikeyBuilder
}

func (tc *apikeyTestcase) setup(t *testing.T) {
	var err error
	if tc.client == nil {
		if tc.transport == nil {
			tc.transport = estest.NewTransport(t, http.StatusOK, map[string]interface{}{
				"application": map[string]interface{}{
					application: map[string]privileges{
						"foo": {PrivilegeAgentConfigRead: true, PrivilegeEventWrite: true, PrivilegeSourcemapWrite: false},
						"bar": {PrivilegeAgentConfigRead: true, PrivilegeEventWrite: false},
					}}})
		}
		tc.client, err = estest.NewElasticsearchClient(tc.transport)
		require.NoError(t, err)
	}
	if tc.cache == nil {
		tc.cache = newPrivilegesCache(time.Millisecond, 5)
	}
	if tc.anyOfPrivileges == nil {
		tc.anyOfPrivileges = []string{PrivilegeEventWrite, PrivilegeSourcemapWrite}
	}
	tc.builder = newApikeyBuilder(tc.client, tc.cache, tc.anyOfPrivileges)
}
