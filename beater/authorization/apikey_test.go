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
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/apmtest"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/elasticsearch/estest"
)

func TestApikeyBuilder(t *testing.T) {
	// in case handler does not read from cache, but from ES an error is returned
	tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}

	tc.setup(t)
	key := "myApiKey"
	handler1 := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", key)
	handler2 := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", key)

	// add existing privileges to shared cache
	privilegesValid := elasticsearch.Permissions{}
	for _, p := range PrivilegesAll {
		privilegesValid[p.Action] = true
	}
	resource := Resource{}
	tc.cache.add(id(key, "-"), privilegesValid)

	// check that cache is actually shared between apiKeyHandlers
	result, err := handler1.AuthorizedFor(context.Background(), resource)
	assert.NoError(t, err)
	assert.Equal(t, Result{Authorized: true}, result)

	result, err = handler2.AuthorizedFor(context.Background(), resource)
	assert.NoError(t, err)
	assert.Equal(t, Result{Authorized: true}, result)
}

func TestAPIKey_AuthorizedFor(t *testing.T) {
	t.Run("cache full", func(t *testing.T) {
		tc := &apikeyTestcase{apiKeyLimit: 1}
		tc.setup(t)

		handler1 := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "foo")
		for i := 0; i < 2; i++ {
			result, err := handler1.AuthorizedFor(context.Background(), Resource{})
			assert.Equal(t, Result{Authorized: true}, result)
			assert.NoError(t, err)
		}

		handler2 := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "bar")
		result, err := handler2.AuthorizedFor(context.Background(), Resource{})
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Error(t, err) // cache is full
	})

	t.Run("from cache", func(t *testing.T) {
		// in case handler does not read from cache, but from ES an error is returned
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}
		tc.setup(t)

		keyValid := "foo"
		keyInvalid := "bar"
		keyMissing := "missing"
		tc.cache.add(id(keyValid, "-"), elasticsearch.Permissions{PrivilegeEventWrite.Action: true})
		tc.cache.add(id(keyInvalid, "-"), elasticsearch.Permissions{PrivilegeEventWrite.Action: false})

		handlerValid := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", keyValid)
		handlerInvalid := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", keyInvalid)
		handlerMissing := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", keyMissing)

		result, err := handlerValid.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: true}, result)

		result, err = handlerInvalid.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)

		result, err = handlerMissing.AuthorizedFor(context.Background(), Resource{})
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
	})

	t.Run("from ES", func(t *testing.T) {
		tc := &apikeyTestcase{}
		tc.setup(t)

		handler := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "key1")
		result, err := handler.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: true}, result)

		handler = tc.builder.ForPrivilege(PrivilegeSourcemapWrite.Action).AuthorizationFor("ApiKey", "key2")
		result, err = handler.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)

		handler = tc.builder.ForPrivilege("missing").AuthorizationFor("ApiKey", "key3")
		result, err = handler.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 3, tc.cache.cache.ItemCount())
	})

	t.Run("client error", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, -1, nil)}
		tc.setup(t)
		handler := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "12a3")

		result, err := handler.AuthorizedFor(context.Background(), Resource{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client error")
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})

	t.Run("unauthorized status from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusUnauthorized, nil)}
		tc.setup(t)
		handler := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "12a3")

		result, err := handler.AuthorizedFor(context.Background(), Resource{})
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 1, tc.cache.cache.ItemCount()) // unauthorized responses are cached
	})

	t.Run("invalid status from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusNotFound, nil)}
		tc.setup(t)
		handler := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "12a3")

		result, err := handler.AuthorizedFor(context.Background(), Resource{})
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 0, tc.cache.cache.ItemCount())
	})

	t.Run("decode error from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusOK, nil)}
		tc.setup(t)
		handler := tc.builder.ForPrivilege(PrivilegeEventWrite.Action).AuthorizationFor("ApiKey", "123")
		result, err := handler.AuthorizedFor(context.Background(), Resource{})
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})
}

type apikeyTestcase struct {
	transport   *estest.Transport
	client      elasticsearch.Client
	cache       *privilegesCache
	apiKeyLimit int

	builder *Builder
}

func (tc *apikeyTestcase) setup(t *testing.T) {
	var err error
	if tc.client == nil {
		if tc.transport == nil {
			tc.transport = estest.NewTransport(t, http.StatusOK, map[string]interface{}{
				"application": map[string]interface{}{
					"apm": map[string]map[string]interface{}{
						"-": {"config_agent:read": true, "event:write": true, "sourcemap:write": false},
					},
				},
			})
		}
		tc.client, err = estest.NewElasticsearchClient(tc.transport)
		require.NoError(t, err)
	}

	cfg := config.DefaultConfig()
	cfg.AgentAuth.APIKey.Enabled = true
	if tc.apiKeyLimit > 0 {
		cfg.AgentAuth.APIKey.LimitPerMin = tc.apiKeyLimit
	}
	tc.builder, err = NewBuilder(cfg.AgentAuth)
	require.NoError(t, err)
	tc.builder.apikey.esClient = tc.client
	tc.cache = tc.builder.apikey.cache
}

func TestApikeyBuilderTraceContext(t *testing.T) {
	transport := estest.NewTransport(t, http.StatusOK, map[string]interface{}{})
	client, err := estest.NewElasticsearchClient(transport)
	require.NoError(t, err)

	cache := newPrivilegesCache(time.Minute, 5)
	anyOfPrivileges := []elasticsearch.PrivilegeAction{PrivilegeEventWrite.Action, PrivilegeSourcemapWrite.Action}
	builder := newApikeyBuilder(client, cache, anyOfPrivileges)
	handler := builder.forKey("12a3")

	_, spans, _ := apmtest.WithTransaction(func(ctx context.Context) {
		// When AuthorizedFor is called with a context containing
		// a transaction, the underlying Elasticsearch query should
		// create a span.
		handler.AuthorizedFor(ctx, Resource{})
		handler.AuthorizedFor(ctx, Resource{}) // cached, no query
	})
	require.Len(t, spans, 1)
	assert.Equal(t, "elasticsearch", spans[0].Subtype)
}
