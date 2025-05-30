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

package asserts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/managedby"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
)

func TestCheckDataStreams(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string]asserts.DataStreamExpectation
		dss      []types.DataStream
	}{
		{
			name: "default",
			expected: map[string]asserts.DataStreamExpectation{
				"logs-apm.error-default": {
					DSManagedBy:      "Index Lifecycle Management",
					PreferIlm:        true,
					IndicesManagedBy: []string{"Data Stream Lifecycle", "Index Lifecycle Management"},
				},
				"metrics-apm.app.opbeans_python-default": {
					DSManagedBy:      "Index Lifecycle Management",
					PreferIlm:        true,
					IndicesManagedBy: []string{"Data Stream Lifecycle", "Index Lifecycle Management"},
				},
			},
			dss: []types.DataStream{
				{
					Name:                    "logs-apm.error-default",
					PreferIlm:               true,
					NextGenerationManagedBy: managedby.ManagedBy{Name: "Index Lifecycle Management"},
					Indices: []types.DataStreamIndex{
						{
							IndexName: ".ds-logs-apm.error-default-2025.03.22-000001",
							ManagedBy: &managedby.ManagedBy{Name: "Data Stream Lifecycle"},
						},
						{
							IndexName: ".ds-logs-apm.error-default-2025.03.22-000002",
							ManagedBy: &managedby.ManagedBy{Name: "Index Lifecycle Management"},
						},
					},
				},
				{
					Name:                    "metrics-apm.app.opbeans_python-default",
					PreferIlm:               true,
					NextGenerationManagedBy: managedby.ManagedBy{Name: "Index Lifecycle Management"},
					Indices: []types.DataStreamIndex{
						{
							IndexName: ".ds-metrics-apm.app.opbeans_python-default-2025.03.22-000001",
							ManagedBy: &managedby.ManagedBy{Name: "Data Stream Lifecycle"},
						},
						{
							IndexName: ".ds-metrics-apm.app.opbeans_python-default-2025.03.22-000002",
							ManagedBy: &managedby.ManagedBy{Name: "Index Lifecycle Management"},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asserts.DataStreamsMeetExpectation(t, tt.expected, tt.dss)
			assert.False(t, t.Failed())
		})
	}
}
