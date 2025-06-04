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

package asserts

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/functionaltests/internal/elasticsearch"
)

func TestDocCountIncreased(t *testing.T) {
	type args struct {
		currDocCount elasticsearch.DataStreamsDocCount
		prevDocCount elasticsearch.DataStreamsDocCount
		expectedDiff elasticsearch.DataStreamsDocCount
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "default",
			args: args{
				currDocCount: elasticsearch.DataStreamsDocCount{
					"traces-apm-default":                     100,
					"logs-apm.error-default":                 200,
					"metrics-apm.app.opbeans_python-default": 12,
				},
				prevDocCount: elasticsearch.DataStreamsDocCount{
					"traces-apm-default":                     50,
					"logs-apm.error-default":                 100,
					"metrics-apm.app.opbeans_python-default": 10,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DocCountIncreased(t, tt.args.currDocCount, tt.args.prevDocCount)
			assert.False(t, t.Failed())
		})
	}
}
