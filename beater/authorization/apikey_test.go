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
	privilegesValid := elasticsearch.Permissions{}
	for _, p := range PrivilegesAll {
		privilegesValid[p.Action] = true
	}
	resource := elasticsearch.Resource("service-go")
	tc.cache.add(id(key, resource), privilegesValid)

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
		tc := &apikeyTestcase{cache: newPrivilegesCache(time.Millisecond, 1)}
		tc.setup(t)
		handler := tc.builder.forKey("")

		result, err := handler.AuthorizedFor(context.Background(), "data:ingest")
		assert.Equal(t, Result{Authorized: false}, result)
		assert.NoError(t, err)

		result, err = handler.AuthorizedFor(context.Background(), "apm:read")
		assert.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
	})

	t.Run("from cache", func(t *testing.T) {
		// in case handler does not read from cache, but from ES an error is returned
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusInternalServerError, nil)}
		tc.setup(t)
		key := ""
		handler := tc.builder.forKey(key)
		resourceValid := elasticsearch.Resource("foo")
		resourceInvalid := elasticsearch.Resource("bar")
		resourceMissing := elasticsearch.Resource("missing")

		tc.cache.add(id(key, resourceValid), elasticsearch.Permissions{tc.anyOfPrivileges[0]: true})
		tc.cache.add(id(key, resourceInvalid), elasticsearch.Permissions{tc.anyOfPrivileges[0]: false})

		result, err := handler.AuthorizedFor(context.Background(), resourceValid)
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: true}, result)

		result, err = handler.AuthorizedFor(context.Background(), resourceInvalid)
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)

		result, err = handler.AuthorizedFor(context.Background(), resourceMissing)
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
	})

	t.Run("from ES", func(t *testing.T) {
		tc := &apikeyTestcase{}
		tc.setup(t)
		handler := tc.builder.forKey("key")

		result, err := handler.AuthorizedFor(context.Background(), "foo")
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: true}, result)

		result, err = handler.AuthorizedFor(context.Background(), "bar")
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)

		result, err = handler.AuthorizedFor(context.Background(), "missing")
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 3, tc.cache.cache.ItemCount())
	})

	t.Run("client error", func(t *testing.T) {
		tc := &apikeyTestcase{
			transport: estest.NewTransport(t, -1, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("12a3")

		result, err := handler.AuthorizedFor(context.Background(), "xyz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client error")
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})

	t.Run("unauthorized status from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusUnauthorized, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("12a3")

		result, err := handler.AuthorizedFor(context.Background(), "xyz")
		require.NoError(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 1, tc.cache.cache.ItemCount()) // unauthorized responses are cached
	})

	t.Run("invalid status from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusNotFound, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("12a3")

		result, err := handler.AuthorizedFor(context.Background(), "xyz")
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Equal(t, 0, tc.cache.cache.ItemCount())
	})

	t.Run("decode error from ES", func(t *testing.T) {
		tc := &apikeyTestcase{transport: estest.NewTransport(t, http.StatusOK, nil)}
		tc.setup(t)
		handler := tc.builder.forKey("123")
		result, err := handler.AuthorizedFor(context.Background(), "foo")
		require.Error(t, err)
		assert.Equal(t, Result{Authorized: false}, result)
		assert.Zero(t, tc.cache.cache.ItemCount())
	})
}

type apikeyTestcase struct {
	transport       *estest.Transport
	client          elasticsearch.Client
	cache           *privilegesCache
	anyOfPrivileges []elasticsearch.PrivilegeAction

	builder *apikeyBuilder
}

func (tc *apikeyTestcase) setup(t *testing.T) {
	var err error
	if tc.client == nil {
		if tc.transport == nil {
			tc.transport = estest.NewTransport(t, http.StatusOK, map[string]interface{}{
				"application": map[string]interface{}{
					"apm": map[string]map[string]interface{}{
						"foo": {"config_agent:read": true, "event:write": true, "sourcemap:write": false},
						"bar": {"config_agent:read": true, "event:write": false},
					}}})
		}
		tc.client, err = estest.NewElasticsearchClient(tc.transport)
		require.NoError(t, err)
	}
	if tc.cache == nil {
		tc.cache = newPrivilegesCache(time.Minute, 5)
	}
	if tc.anyOfPrivileges == nil {
		tc.anyOfPrivileges = []elasticsearch.PrivilegeAction{PrivilegeEventWrite.Action, PrivilegeSourcemapWrite.Action}
	}
	tc.builder = newApikeyBuilder(tc.client, tc.cache, tc.anyOfPrivileges)
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
		handler.AuthorizedFor(ctx, ResourceInternal)
		handler.AuthorizedFor(ctx, ResourceInternal) // cached, no query
	})
	require.Len(t, spans, 1)
	assert.Equal(t, "elasticsearch", spans[0].Subtype)
}
