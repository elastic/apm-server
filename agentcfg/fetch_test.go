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

package agentcfg

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/kibana"

	"github.com/elastic/apm-server/tests"
)

type m map[string]interface{}

var cfg = Config{CacheExpiration: time.Second}

func TestMain(m *testing.M) {
	//ensure cache is cleared out after test runs
	code := m.Run()
	if docCache != nil {
		docCache.gocache.Flush()
	}
	os.Exit(code)
}

func query(name string) Query {
	return Query{Service: Service{Name: name}}
}

func TestFetchNoClient(t *testing.T) {
	kb, kerr := kibana.NewKibanaClient(nil)
	_, _, ferr := Fetch(kb, &cfg, query(t.Name()), kerr)
	require.Error(t, ferr)
	assert.Equal(t, kerr, ferr)
}

func TestFetchStringConversion(t *testing.T) {
	kb := tests.MockKibana(http.StatusOK,
		m{
			"_id": "1",
			"_source": m{
				"settings": m{
					"sampling_rate": 0.5,
				},
			},
		})
	result, etag, err := Fetch(kb, &cfg, query(t.Name()), nil)
	require.NoError(t, err)
	assert.Equal(t, "1", etag, etag)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)
}

func TestFetchVersionCheck(t *testing.T) {
	kb := tests.MockKibana(http.StatusOK, m{})
	kb.Connection.Version.Major = 6
	_, _, err := Fetch(kb, &cfg, query(t.Name()), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestFetchError(t *testing.T) {
	kb := tests.MockKibana(http.StatusNotFound, m{"error": "an error"})
	_, _, err := Fetch(kb, &cfg, query(t.Name()), nil)
	require.Error(t, err)
	assert.Equal(t, "{\"error\":\"an error\"}", err.Error())
}

func TestFetchWithCaching(t *testing.T) {
	q := query(t.Name())
	fetch := func(samplingRate float64) map[string]string {
		client := func(samplingRate float64) *kibana.Client {
			return tests.MockKibana(http.StatusOK,
				m{
					"_id": "1",
					"_source": m{
						"settings": m{
							"sampling_rate": samplingRate,
						},
					},
				})
		}

		result, id, err := Fetch(client(samplingRate), &cfg, q, nil)
		require.NoError(t, err)
		require.Equal(t, "1", id)
		return result
	}

	// nothing cached yet
	result := fetch(0.5)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)

	// next fetch runs against cache
	result = fetch(0.8)
	assert.Equal(t, map[string]string{"sampling_rate": "0.5"}, result)

	// fetch from Kibana again
	docCache.gocache.Delete(q.id())
	result = fetch(0.7)
	assert.Equal(t, map[string]string{"sampling_rate": "0.7"}, result)

}
