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

package sourcemap

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-sourcemap/sourcemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"

	"github.com/elastic/apm-server/elasticsearch/estest"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/sourcemap/test"
)

func Test_esFetcher_fetchError(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode int
		esBody     map[string]interface{}
		temporary  bool
	}{
		"es not reachable": {
			statusCode: -1, temporary: true,
		},
		"es bad request": {
			statusCode: http.StatusBadRequest,
		},
		"empty sourcemap string": {
			esBody: map[string]interface{}{
				"hits": map[string]interface{}{
					"total": map[string]interface{}{"value": 1},
					"hits": []map[string]interface{}{
						{"_source": map[string]interface{}{
							"sourcemap": map[string]interface{}{
								"sourcemap": ""}}}}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			statusCode := tc.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			client, err := estest.NewElasticsearchClient(estest.NewTransport(t, statusCode, tc.esBody))
			require.NoError(t, err)
			consumer, err := testESStore(client).fetch(context.Background(), "abc", "1.0", "/tmp")
			require.Error(t, err)
			if tc.temporary {
				assert.Contains(t, err.Error(), errMsgESFailure)
			} else {
				assert.NotContains(t, err.Error(), errMsgESFailure)
			}
			assert.Empty(t, consumer)
		})
	}
}

func Test_esFetcher_fetch(t *testing.T) {
	for name, tc := range map[string]struct {
		client   elasticsearch.Client
		filePath string
	}{
		"no sourcemap found":                {client: test.ESClientWithSourcemapNotFound(t)},
		"sourcemap indicated but not found": {client: test.ESClientWithSourcemapIndicatedNotFound(t)},
		"valid sourcemap found":             {client: test.ESClientWithValidSourcemap(t), filePath: "bundle.js"},
	} {
		t.Run(name, func(t *testing.T) {
			sourcemapStr, err := testESStore(tc.client).fetch(context.Background(), "abc", "1.0", "/tmp")
			require.NoError(t, err)

			if tc.filePath == "" {
				assert.Empty(t, sourcemapStr)
			} else {
				sourcemapConsumer, err := sourcemap.Parse("", []byte(sourcemapStr))
				require.NoError(t, err)
				assert.Equal(t, tc.filePath, sourcemapConsumer.File())
			}
		})
	}
}

func testESStore(client elasticsearch.Client) *esStore {
	return &esStore{client: client, index: "apm-sourcemap", logger: logp.NewLogger(logs.Sourcemap)}
}
