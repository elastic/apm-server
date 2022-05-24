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

package modelindexertest

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelindexer"
)

// AppendEncodedBatch encodes batch to JSON-encoded documents, appending them
// to out and returning the extended slice. AppendEncodedBatch will fail the
// test if event encoding incurs any errors.
func AppendEncodedBatch(t testing.TB, out [][]byte, batch model.Batch) [][]byte {
	var bulkHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		docs, response := DecodeBulkRequest(r)
		defer json.NewEncoder(w).Encode(response)
		out = append(out, docs...)
	}
	client := NewMockElasticsearchClient(t, bulkHandler)
	indexer, err := modelindexer.New(client, modelindexer.Config{})
	require.NoError(t, err)
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	err = indexer.Close(context.Background())
	require.NoError(t, err)
	return out
}

// DecodeBulkRequest decodes a /_bulk request's body, returning the decoded documents and a response body.
func DecodeBulkRequest(r *http.Request) ([][]byte, elasticsearch.BulkIndexerResponse) {
	body := r.Body
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		r, err := gzip.NewReader(body)
		if err != nil {
			panic(err)
		}
		defer r.Close()
		body = r
	}

	scanner := bufio.NewScanner(body)
	var indexed [][]byte
	var result elasticsearch.BulkIndexerResponse
	for scanner.Scan() {
		action := make(map[string]interface{})
		if err := json.NewDecoder(strings.NewReader(scanner.Text())).Decode(&action); err != nil {
			panic(err)
		}
		var actionType string
		for actionType = range action {
		}
		if !scanner.Scan() {
			panic("expected source")
		}

		doc := append([]byte{}, scanner.Bytes()...)
		if !json.Valid(doc) {
			panic(fmt.Errorf("invalid JSON: %s", doc))
		}
		indexed = append(indexed, doc)

		item := esutil.BulkIndexerResponseItem{Status: http.StatusCreated}
		result.Items = append(result.Items, map[string]esutil.BulkIndexerResponseItem{actionType: item})
	}
	return indexed, result
}

// NewMockElasticsearchClient returns an elasticsearch.Client which sends /_bulk requests to bulkHandler.
func NewMockElasticsearchClient(t testing.TB, bulkHandler http.HandlerFunc) elasticsearch.Client {
	config := NewMockElasticsearchClientConfig(t, bulkHandler)
	client, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	return client
}

// NewMockElasticsearchClientConfig starts an httptest.Server, and returns an *elasticsearch.Config which
// sends /_bulk requests to bulkHandler. The httptest.Server will be closed via t.Cleanup.
func NewMockElasticsearchClientConfig(t testing.TB, bulkHandler http.HandlerFunc) *elasticsearch.Config {
	mux := http.NewServeMux()
	HandleBulk(mux, bulkHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	config := elasticsearch.DefaultConfig()
	config.Hosts = elasticsearch.Hosts{srv.URL}
	config.Backoff.Max = time.Nanosecond
	return config
}

// HandleBulk registers bulkHandler with mux for handling /_bulk requests,
// wrapping bulkHandler to conform with go-elasticsearch version checking.
func HandleBulk(mux *http.ServeMux, bulkHandler http.HandlerFunc) {
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		bulkHandler.ServeHTTP(w, r)
	})
}
