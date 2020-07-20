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
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/systemtest/estest"
)

const (
	defaultElasticsearchHost = "localhost"
	defaultElasticsearchPort = "9200"
	adminElasticsearchUser   = "admin"
	adminElasticsearchPass   = "changeme"
	maxElasticsearchBackoff  = time.Second
)

var (
	// Elasticsearch is an Elasticsearch client for use in tests.
	Elasticsearch *estest.Client
)

func init() {
	hostport := net.JoinHostPort(defaultElasticsearchHost, defaultElasticsearchPort)
	esURL := url.URL{Scheme: "http", Host: hostport}
	cfg := elasticsearch.Config{
		Addresses: []string{esURL.String()},
		Username:  adminElasticsearchUser,
		Password:  adminElasticsearchPass,
		RetryBackoff: func(attempt int) time.Duration {
			backoff := time.Duration(attempt*100) * time.Millisecond
			if backoff > maxElasticsearchBackoff {
				backoff = maxElasticsearchBackoff
			}
			return backoff
		},
		/*
			Logger: &estransport.ColorLogger{
				Output:             os.Stdout,
				EnableRequestBody:  true,
				EnableResponseBody: true,
			},
		*/
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	Elasticsearch = &estest.Client{Client: client}
}

// CleanupElasticsearch deletes all indices, index templates,
// and ingest node pipelines whose names start with "apm",
// and deletes the default ILM policy "apm-rollover-30-days".
func CleanupElasticsearch(t testing.TB) {
	const prefix = "apm*"
	requests := []estest.Request{
		esapi.IndicesDeleteRequest{Index: []string{prefix}},
		esapi.IngestDeletePipelineRequest{PipelineID: prefix},
		esapi.IndicesDeleteTemplateRequest{Name: prefix},
	}

	doReq := func(req estest.Request) error {
		_, err := Elasticsearch.Do(context.Background(), req, nil)
		if err, ok := err.(*estest.Error); ok && err.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}

	var g errgroup.Group
	for _, req := range requests {
		req := req // copy for closure
		g.Go(func() error { return doReq(req) })
	}
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}

	// Delete the ILM policy last or we'll get an error due to it being in use.
	err := doReq(esapi.ILMDeleteLifecycleRequest{Policy: "apm-rollover-30-days"})
	if err != nil {
		t.Fatal(err)
	}
}
