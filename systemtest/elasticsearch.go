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

package systemtest

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/esutil"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

const (
	adminElasticsearchUser  = "admin"
	adminElasticsearchPass  = "changeme"
	maxElasticsearchBackoff = time.Second
)

var (
	// Elasticsearch is an Elasticsearch client for use in tests.
	Elasticsearch *estest.Client
)

func init() {
	cfg := newElasticsearchConfig()
	cfg.Username = adminElasticsearchUser
	cfg.Password = adminElasticsearchPass
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	Elasticsearch = &estest.Client{Client: client}
}

// NewElasticsearchClientWithAPIKey returns a new estest.Client,
// configured to use apiKey for authentication.
func NewElasticsearchClientWithAPIKey(apiKey string) *estest.Client {
	cfg := newElasticsearchConfig()
	cfg.APIKey = apiKey
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	return &estest.Client{Client: client}
}

func newElasticsearchConfig() elasticsearch.Config {
	var addresses []string
	for _, host := range apmservertest.DefaultConfig().Output.Elasticsearch.Hosts {
		u := url.URL{Scheme: "http", Host: host}
		addresses = append(addresses, u.String())
	}
	return elasticsearch.Config{
		Addresses: addresses,
		RetryBackoff: func(attempt int) time.Duration {
			backoff := time.Duration(attempt*100) * time.Millisecond
			if backoff > maxElasticsearchBackoff {
				backoff = maxElasticsearchBackoff
			}
			return backoff
		},
	}
}

// CleanupElasticsearch deletes all indices, index templates,
// and ingest node pipelines whose names start with "apm",
// and deletes the default ILM policy "apm-rollover-30-days".
func CleanupElasticsearch(t testing.TB) {
	const (
		legacyPrefix     = "apm*" // Not "apm-*", as that would not capture the "apm" ingest pipeline.
		apmTracesPrefix  = "traces-apm*"
		apmMetricsPrefix = "metrics-apm*"
		apmLogsPrefix    = "logs-apm*"
	)

	doReq := func(req estest.Request) error {
		_, err := Elasticsearch.Do(context.Background(), req, nil)
		if err, ok := err.(*estest.Error); ok && err.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}

	doParallel := func(requests ...estest.Request) {
		t.Helper()
		var g errgroup.Group
		for _, req := range requests {
			req := req // copy for closure
			g.Go(func() error { return doReq(req) })
		}
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
	}

	// Delete indices, data streams, and ingest pipelines.
	doReq(esapi.IndicesDeleteRequest{Index: []string{legacyPrefix}})
	doParallel(
		esapi.IndicesDeleteDataStreamRequest{Name: legacyPrefix},
		esapi.IndicesDeleteDataStreamRequest{Name: apmTracesPrefix},
		esapi.IndicesDeleteDataStreamRequest{Name: apmMetricsPrefix},
		esapi.IndicesDeleteDataStreamRequest{Name: apmLogsPrefix},
		esapi.IngestDeletePipelineRequest{PipelineID: legacyPrefix},
	)

	// Delete index templates after deleting data streams.
	doParallel(
		esapi.IndicesDeleteTemplateRequest{Name: legacyPrefix},
		esapi.IndicesDeleteIndexTemplateRequest{Name: apmTracesPrefix},
		esapi.IndicesDeleteIndexTemplateRequest{Name: apmMetricsPrefix},
		esapi.IndicesDeleteIndexTemplateRequest{Name: apmLogsPrefix},
	)

	// Refresh indices to ensure all recent changes are visible.
	if err := doReq(esapi.IndicesRefreshRequest{}); err != nil {
		t.Fatal(err)
	}

	// Delete index templates after deleting data streams.
	if err := doReq(esapi.IndicesDeleteIndexTemplateRequest{Name: legacyPrefix}); err != nil {
		t.Fatal(err)
	}

	// Delete the ILM policy last or we'll get an error due to it being in use.
	for {
		err := doReq(esapi.ILMDeleteLifecycleRequest{Policy: "apm-rollover-30-days"})
		if err == nil {
			break
		}
		// Retry deleting, in case indices are still being deleted.
		const delay = 100 * time.Millisecond
		t.Logf("failed to delete ILM policy (retrying in %s): %s", delay, err)
		time.Sleep(delay)
	}
}

// ChangeUserPassword changes the password for a given user.
func ChangeUserPassword(t testing.TB, username, password string) {
	req := esapi.SecurityChangePasswordRequest{
		Username: username,
		Body:     esutil.NewJSONReader(map[string]interface{}{"password": password}),
	}
	if _, err := Elasticsearch.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}

// InvalidateAPIKeys invalidates all API Keys for the apm-server user.
func InvalidateAPIKeys(t testing.TB) {
	req := esapi.SecurityInvalidateAPIKeyRequest{
		Body: esutil.NewJSONReader(map[string]interface{}{
			"username": apmservertest.DefaultConfig().Output.Elasticsearch.Username,
		}),
	}
	if _, err := Elasticsearch.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}

// InvalidateAPIKey invalidates the API Key with the given ID.
func InvalidateAPIKey(t testing.TB, id string) {
	req := esapi.SecurityInvalidateAPIKeyRequest{
		Body: esutil.NewJSONReader(map[string]interface{}{"id": id}),
	}
	if _, err := Elasticsearch.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}
