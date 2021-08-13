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

package modelindexer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelindexer"
)

func TestModelIndexerIntegration(t *testing.T) {
	switch strings.ToLower(os.Getenv("INTEGRATION_TESTS")) {
	case "1", "true":
	default:
		t.Skip("Skipping integration test, export INTEGRATION_TESTS=1 to run")
	}

	config := elasticsearch.DefaultConfig()
	config.Username = "admin"
	config.Password = "changeme"
	client, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Second})
	require.NoError(t, err)
	defer func() {
		_ = indexer.Close()
	}()

	dataStream := model.DataStream{Type: "logs", Dataset: "apm_server", Namespace: "testing"}
	index := fmt.Sprintf("%s-%s-%s", dataStream.Type, dataStream.Dataset, dataStream.Namespace)

	deleteIndex := func() {
		resp, err := esapi.IndicesDeleteDataStreamRequest{Name: []string{index}}.Do(context.Background(), client)
		require.NoError(t, err)
		defer resp.Body.Close()
	}
	deleteIndex()
	defer deleteIndex()

	const N = 100
	for i := 0; i < N; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: dataStream}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	// Closing the indexer flushes enqueued events.
	err = indexer.Close()
	require.NoError(t, err)

	// Check that docs are indexed.
	resp, err := esapi.IndicesRefreshRequest{Index: []string{index}}.Do(context.Background(), client)
	require.NoError(t, err)
	resp.Body.Close()

	var result struct {
		Count int
	}
	resp, err = esapi.CountRequest{Index: []string{index}}.Do(context.Background(), client)
	require.NoError(t, err)
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, N, result.Count)
}
