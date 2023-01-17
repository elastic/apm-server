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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/kibana"
)

func query(name string) Query {
	return Query{Service: Service{Name: name}, Etag: "123"}
}

func mockDoc(sampleRate float64) map[string]interface{} {
	return map[string]interface{}{
		"_id": "1",
		"_source": map[string]interface{}{
			"settings": map[string]interface{}{
				"sampling_rate": sampleRate,
			},
			"etag":       "123",
			"agent_name": "rum-js",
		},
	}
}

func TestKibanaFetcher(t *testing.T) {
	var statusCode int
	var response map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}))
	defer srv.Close()
	client, err := kibana.NewClient(kibana.ClientConfig{Host: srv.URL})
	require.NoError(t, err)

	t.Run("ExpectationFailed", func(t *testing.T) {
		statusCode = http.StatusExpectationFailed
		response = map[string]interface{}{"error": "an error"}

		kf, err := NewKibanaFetcher(client, testExpiration)
		require.NoError(t, err)
		_, err = kf.Fetch(context.Background(), query(t.Name()))
		require.Error(t, err)
		assert.Equal(t, "agentcfg kibana request failed with status code 417: {\"error\":\"an error\"}\n", err.Error())
	})

	t.Run("NotFound", func(t *testing.T) {
		statusCode = http.StatusNotFound
		response = map[string]interface{}{}

		kf, err := NewKibanaFetcher(client, testExpiration)
		require.NoError(t, err)
		result, err := kf.Fetch(context.Background(), query(t.Name()))
		require.NoError(t, err)
		assert.Equal(t, zeroResult(), result)
	})

	t.Run("Success", func(t *testing.T) {
		statusCode = http.StatusOK
		response = mockDoc(0.5)

		b, err := json.Marshal(response)
		expectedResult, err := newResult(b, err)
		require.NoError(t, err)
		kf, err := NewKibanaFetcher(client, testExpiration)
		require.NoError(t, err)
		result, err := kf.Fetch(context.Background(), query(t.Name()))
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("FetchFromCache", func(t *testing.T) {

		fetcher, err := NewKibanaFetcher(client, time.Minute)
		require.NoError(t, err)
		fetch := func(kibanaSamplingRate, expectedSamplingRate float64) {
			statusCode = http.StatusOK
			response = mockDoc(kibanaSamplingRate)

			b, err := json.Marshal(mockDoc(expectedSamplingRate))
			require.NoError(t, err)
			expectedResult, err := newResult(b, err)
			require.NoError(t, err)

			result, err := fetcher.Fetch(context.Background(), query(t.Name()))
			require.NoError(t, err)
			assert.Equal(t, expectedResult, result)
		}

		// nothing cached yet
		fetch(0.5, 0.5)

		// next fetch runs against cache
		fetch(0.8, 0.5)

		// after key is expired, fetch from Kibana again
		fetcher.cache.gocache.Delete(query(t.Name()).id())
		fetch(0.7, 0.7)

	})
}
