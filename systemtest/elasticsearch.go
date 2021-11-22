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
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

const (
	adminElasticsearchUser  = "admin"
	adminElasticsearchPass  = "changeme"
	maxElasticsearchBackoff = 10 * time.Second
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
		Addresses:  addresses,
		MaxRetries: 5,
		RetryBackoff: func(attempt int) time.Duration {
			backoff := (500 * time.Millisecond) * (1 << (attempt - 1))
			if backoff > maxElasticsearchBackoff {
				backoff = maxElasticsearchBackoff
			}
			return backoff
		},
	}
}

// CleanupElasticsearch deletes all data streams created by APM Server.
func CleanupElasticsearch(t testing.TB) {
	_, err := Elasticsearch.Do(context.Background(), &esapi.IndicesDeleteDataStreamRequest{Name: []string{
		"traces-apm*",
		"metrics-apm*",
		"logs-apm*",
	}}, nil)
	require.NoError(t, err)
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
