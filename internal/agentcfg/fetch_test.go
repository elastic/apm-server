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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testExpiration = time.Nanosecond
)

func TestCustomJSON(t *testing.T) {
	expected := Result{Source: Source{
		Etag:     "123",
		Settings: map[string]string{"transaction_sampling_rate": "0.3"}}}
	input := `{"_id": "1", "_source":{"etag":"123", "settings":{"transaction_sampling_rate": 0.3}}}`
	actual, _ := newResult([]byte(input), nil)
	assert.Equal(t, expected, actual)
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
					AgentName:   "Jaeger/Python",
					Config:      map[string]string{"key1": "val1", "key2": "val2", "transaction_sample_rate": "0.1"},
					Etag:        "abc123",
				},
			},
			expectedSettings: map[string]string{
				"key1":                    "val1",
				"key2":                    "val2",
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
