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

var (
	testExpiration = time.Nanosecond
)

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

		_, err := NewKibanaFetcher(client, testExpiration).Fetch(context.Background(), query(t.Name()))
		require.Error(t, err)
		assert.Equal(t, "{\"error\":\"an error\"}"+"\n", err.Error())
	})

	t.Run("NotFound", func(t *testing.T) {
		statusCode = http.StatusNotFound
		response = map[string]interface{}{}

		result, err := NewKibanaFetcher(client, testExpiration).Fetch(context.Background(), query(t.Name()))
		require.NoError(t, err)
		assert.Equal(t, zeroResult(), result)
	})

	t.Run("Success", func(t *testing.T) {
		statusCode = http.StatusOK
		response = mockDoc(0.5)

		b, err := json.Marshal(response)
		expectedResult, err := newResult(b, err)
		require.NoError(t, err)
		result, err := NewKibanaFetcher(client, testExpiration).Fetch(context.Background(), query(t.Name()))
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("FetchFromCache", func(t *testing.T) {

		fetcher := NewKibanaFetcher(client, time.Minute)
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

func TestSanitize(t *testing.T) {
	input := Result{Source: Source{
		Agent:    "python",
		Settings: Settings{"transaction_sample_rate": "0.1", "capture_body": "false"}}}
	// full result as not requested for an insecure agent
	res := sanitize([]string{}, input)
	assert.Equal(t, input, res)

	// no result for insecure agent
	res = sanitize([]string{"rum-js"}, input)
	assert.Equal(t, zeroResult(), res)

	// limited result for insecure agent
	insecureAgents := []string{"rum-js"}
	input.Source.Agent = "rum-js"
	assert.Equal(t, Settings{"transaction_sample_rate": "0.1"}, sanitize(insecureAgents, input).Source.Settings)

	// limited result for insecure agent prefix
	insecureAgents = []string{"Jaeger"}
	input.Source.Agent = "Jaeger/Python"
	assert.Equal(t, Settings{"transaction_sample_rate": "0.1"}, sanitize(insecureAgents, input).Source.Settings)

	// no result for insecure agent prefix
	insecureAgents = []string{"Python"}
	input.Source.Agent = "Jaeger/Python"
	res = sanitize(insecureAgents, input)
	assert.Equal(t, zeroResult(), res)
}

func TestCustomJSON(t *testing.T) {
	expected := Result{Source: Source{
		Etag:     "123",
		Settings: map[string]string{"transaction_sampling_rate": "0.3"}}}
	input := `{"_id": "1", "_source":{"etag":"123", "settings":{"transaction_sampling_rate": 0.3}}}`
	actual, _ := newResult([]byte(input), nil)
	assert.Equal(t, expected, actual)
}

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

func TestDirectConfigurationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		query            Query
		agentConfigs     []AgentConfig
		expectedSettings map[string]string
	}{
		{
			query: Query{
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key1": "val2", "key2": "val2"},
					Etag:               "def456",
				},
				{
					ServiceName: "service1",
					Config:      map[string]string{"key3": "val3"},
					Etag:        "abc123",
				},
				{
					ServiceName:        "service1",
					ServiceEnvironment: "production",
					Config:             map[string]string{"key1": "val1"},
					Etag:               "abc123",
				},
			},
			expectedSettings: map[string]string{
				"key1": "val1",
			},
		},
		{
			query: Query{
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key3": "val3"},
					Etag:               "def456",
				},
				{
					ServiceName: "service1",
					Config:      map[string]string{"key1": "val1", "key2": "val2"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			query: Query{
				InsecureAgents: []string{"Jaeger"},
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key3": "val3"},
					Etag:               "def456",
				},
				{
					ServiceName: "service1",
					Config:      map[string]string{"key1": "val1", "key2": "val2"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{},
		},
		{
			query: Query{
				InsecureAgents: []string{"Jaeger"},
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key3": "val3"},
					Etag:               "def456",
				},
				{
					ServiceName: "service1",
					AgentName:   "Jaeger/Python",
					Config:      map[string]string{"key1": "val1", "key2": "val2", "transaction_sample_rate": "0.1"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{
				"transaction_sample_rate": "0.1",
			},
		},
		{
			query: Query{
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceName: "service2",
					Config:      map[string]string{"key1": "val1", "key2": "val2"},
					Etag:        "abc123",
				},
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key3": "val3"},
					Etag:               "def456",
				},
			},
			expectedSettings: map[string]string{
				"key3": "val3",
			},
		},
		{
			query: Query{
				Service: Service{
					Name:        "service1",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceName: "not-found",
					Config:      map[string]string{"key1": "val1"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{},
		},
		{
			query: Query{
				Service: Service{
					Name:        "service2",
					Environment: "production",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceName: "service1",
					Config:      map[string]string{"key1": "val1", "key2": "val2"},
					Etag:        "abc123",
				},
				{
					ServiceName: "service2",
					Config:      map[string]string{"key1": "val4", "key2": "val5"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{
				"key1": "val4",
				"key2": "val5",
			},
		},
		{
			query: Query{
				Service: Service{
					Name:        "service2",
					Environment: "staging",
				},
			},
			agentConfigs: []AgentConfig{
				{
					ServiceName: "service1",
					Config:      map[string]string{"key1": "val1", "key2": "val2"},
					Etag:        "abc123",
				},
				{
					ServiceEnvironment: "production",
					Config:             map[string]string{"key1": "val4", "key2": "val5"},
					Etag:               "abc123",
				},
				{
					Config: map[string]string{"key3": "val5", "key4": "val6"},
					Etag:   "abc123",
				},
			},
			expectedSettings: map[string]string{
				"key3": "val5",
				"key4": "val6",
			},
		},
	} {
		f := NewDirectFetcher(tc.agentConfigs)
		result, err := f.Fetch(context.Background(), tc.query)
		require.NoError(t, err)

		assert.Equal(t, Settings(tc.expectedSettings), result.Source.Settings)
	}
}
