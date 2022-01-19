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

package elasticsearch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	// Imported to ensure the type data is available for reflection.
	_ "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"

	"github.com/modern-go/reflect2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set HTTP_PROXY at package init time, because
	// http.ProxyFromEnvironment is cached and changes
	// to the environment will not affect it later.
	os.Setenv("HTTP_PROXY", "proxy.invalid")
}

func TestHttpProxyUrl(t *testing.T) {
	t.Run("proxy disabled", func(t *testing.T) {
		proxy, err := httpProxyURL(&Config{ProxyDisable: true})
		require.Nil(t, err)
		assert.Nil(t, proxy)
	})

	t.Run("proxy from ENV", func(t *testing.T) {
		// create proxy function
		proxy, err := httpProxyURL(&Config{})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err := proxy(httptest.NewRequest(http.MethodGet, "http://example.com", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://proxy.invalid", url.String())
	})

	t.Run("proxy from URL", func(t *testing.T) {
		// create proxy function from URL without `http` prefix
		proxy, err := httpProxyURL(&Config{ProxyURL: "foo"})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err := proxy(httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://foo", url.String())

		// create proxy function from URL with `http` prefix
		proxy, err = httpProxyURL(&Config{ProxyURL: "http://foo"})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err = proxy(httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://foo", url.String())
	})
}

func TestAddresses(t *testing.T) {
	t.Run("no protocol and path", func(t *testing.T) {
		addresses, err := addresses(&Config{Hosts: []string{
			"http://localhost", "http://localhost:9300", "localhost", "192.0.0.1", "192.0.0.2:8080"}})
		require.NoError(t, err)
		expected := []string{"http://localhost:9200", "http://localhost:9300",
			"http://localhost:9200", "http://192.0.0.1:9200", "http://192.0.0.2:8080"}
		assert.ElementsMatch(t, expected, addresses)
	})

	t.Run("with protocol and path", func(t *testing.T) {
		addresses, err := addresses(&Config{Protocol: "https", Path: "xyz",
			Hosts: []string{"http://localhost", "http://localhost:9300/abc",
				"localhost/abc", "192.0.0.2:8080"}})
		require.NoError(t, err)
		expected := []string{"http://localhost:9200/xyz", "http://localhost:9300/abc",
			"https://localhost:9200/abc", "https://192.0.0.2:8080/xyz"}
		assert.ElementsMatch(t, expected, addresses)
	})
}

// TestBeatsConfigSynced helps ensure that our elasticsearch.Config struct is
// kept in sync with the config defined in libbeat/outputs/elasticsearch.
func TestBeatsConfigSynced(t *testing.T) {
	libbeatType, _ := reflect2.TypeByPackageName(
		"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch",
		"elasticsearchConfig",
	).(reflect2.StructType)
	require.NotNil(t, libbeatType)

	localType, _ := reflect2.TypeByPackageName(
		"github.com/elastic/apm-server/elasticsearch",
		"Config",
	).(reflect2.StructType)
	require.NotNil(t, localType)

	type structField struct {
		reflect2.StructField
		structTag reflect.StructTag
	}
	getStructFields := func(typ reflect2.StructType) map[string]structField {
		out := make(map[string]structField)
		for i := typ.NumField() - 1; i >= 0; i-- {
			field := structField{StructField: typ.Field(i)}
			field.structTag = field.Tag()
			configTag := strings.Split(field.structTag.Get("config"), ",")
			configName := configTag[0]
			if configName == "" {
				configName = strings.ToLower(field.Name())
			}
			out[configName] = field
		}
		return out
	}

	libbeatStructFields := getStructFields(libbeatType)
	localStructFields := getStructFields(localType)

	// "hosts" is only expected in the local struct
	delete(localStructFields, "hosts")

	// We expect the libbeat struct to be a superset of all other
	// fields defined in the local struct, with identical tags and
	// types. Struct field names do not need to match.
	//
	// TODO(simitt): take a closer look at ES ouput changes in libbeat
	// introduced with https://github.com/elastic/beats/pull/25219
	localStructExceptions := map[string]interface{}{
		"ssl": nil, "timeout": nil, "proxy_disable": nil, "proxy_url": nil}
	for name, localStructField := range localStructFields {
		if _, ok := localStructExceptions[name]; ok {
			continue
		}
		require.Contains(t, libbeatStructFields, name)
		libbeatStructField := libbeatStructFields[name]
		assert.Equal(t, localStructField.structTag, libbeatStructField.structTag)
		assert.Equal(t,
			localStructField.Type(),
			libbeatStructField.Type(),
			fmt.Sprintf("expected type %s for config field %q, got %s",
				libbeatStructField.Type(), name, localStructField.Type(),
			),
		)
		delete(libbeatStructFields, name)
	}

	knownUnhandled := []string{
		"bulk_max_size",
		"escape_html",
		// TODO Kerberos auth (https://github.com/elastic/apm-server/issues/3794)
		"kerberos",
		"loadbalance",
		"parameters",
		"transport",
		"non_indexable_policy",
		"allow_older_versions",
	}
	for name := range libbeatStructFields {
		assert.Contains(t, knownUnhandled, name)
	}
}
