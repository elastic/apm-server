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

package esclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/esql/query"
	"github.com/elastic/go-elasticsearch/v8/typedapi/security/createapikey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type Client struct {
	es *elasticsearch.TypedClient
}

// New returns a new Client for querying APM data.
func New(cfg Config) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.TLSSkipVerify}

	es, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
		Username:  cfg.Username,
		APIKey:    cfg.APIKey,
		Password:  cfg.Password,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch client: %w", err)
	}
	return &Client{
		es: es,
	}, nil
}

var elasticsearchTimeUnits = []struct {
	Duration time.Duration
	Unit     string
}{
	{time.Hour, "h"},
	{time.Minute, "m"},
	{time.Second, "s"},
	{time.Millisecond, "ms"},
	{time.Microsecond, "micros"},
}

// formatDurationElasticsearch formats a duration using
// Elasticsearch supported time units.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/current/api-conventions.html#time-units
func formatDurationElasticsearch(d time.Duration) string {
	for _, tu := range elasticsearchTimeUnits {
		if d%tu.Duration == 0 {
			return fmt.Sprintf("%d%s", d/tu.Duration, tu.Unit)
		}
	}
	return fmt.Sprintf("%dnanos", d)
}

// CreateAgentAPIKey creates an agent API Key, and returns it in the
// base64-encoded form that agents should provide.
//
// If expiration is less than or equal to zero, then the API Key never expires.
func (c *Client) CreateAPIKey(ctx context.Context, name string, expiration time.Duration, roles map[string]types.RoleDescriptor) (string, error) {
	var maybeExpiration types.Duration
	if expiration > 0 {
		maybeExpiration = formatDurationElasticsearch(expiration)
	}
	resp, err := c.es.Security.CreateApiKey().Request(&createapikey.Request{
		Name:            &name,
		Expiration:      maybeExpiration,
		RoleDescriptors: roles,
		Metadata: map[string]json.RawMessage{
			"creator": []byte(`"apmclient"`),
		},
	}).Do(ctx)
	if err != nil {
		return "", fmt.Errorf("error creating API Key: %w", err)
	}
	return resp.Encoded, nil
}

func (c *Client) CreateAPMAPIKey(ctx context.Context, name string) (string, error) {
	return c.CreateAPIKey(context.Background(),
		name, -1, map[string]types.RoleDescriptor{},
	)
}

func (c *Client) GetDataStream(ctx context.Context, name string) ([]types.DataStream, error) {
	resp, err := c.es.Indices.GetDataStream().Name(name).Do(ctx)
	if err != nil {
		return []types.DataStream{}, fmt.Errorf("cannot GET datastream: %w", err)
	}

	return resp.DataStreams, nil
}

type ApmDocCount struct {
	Count      int
	Datastream string
}

func (c *Client) ApmDocCount(ctx context.Context) ([]ApmDocCount, error) {
	q := `FROM traces-apm*,apm-*,traces-*.otel-*,logs-apm*,apm-*,logs-*.otel-*,metrics-apm*,apm-*,metrics-*.otel-*
| EVAL datastream = CONCAT(data_stream.type, "-", data_stream.dataset, "-", data_stream.namespace)
| STATS count = COUNT(*) BY datastream
| SORT count DESC`

	qry := c.es.Esql.Query().Query(q)
	res, err := query.Helper[ApmDocCount](ctx, qry)
	if err != nil {
		return []ApmDocCount{}, fmt.Errorf("cannot retrieve APM doc count: %w", err)
	}

	return res, nil
}