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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/esql/query"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
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

// CreateAPIKey creates an API Key, and returns it in the base64-encoded form
// that agents should provide.
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
		return "", fmt.Errorf("error creating API key: %w", err)
	}
	return resp.Encoded, nil
}

// CreateRerouteProcessors creates re-route processors for logs, metrics and traces.
func (c *Client) CreateRerouteProcessors(ctx context.Context) error {
	for _, id := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		if err := c.createRerouteProcessor(ctx, id, "rerouted"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) createRerouteProcessor(ctx context.Context, id string, name string) error {
	_, err := c.es.Ingest.PutPipeline(id).Request(&putpipeline.Request{
		Processors: []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{name},
				},
			},
		},
	}).Do(ctx)
	if err != nil {
		return fmt.Errorf("error creating reroute processor for %s: %w", id, err)
	}
	return nil
}

func (c *Client) GetDataStream(ctx context.Context, name string) ([]types.DataStream, error) {
	resp, err := c.es.Indices.GetDataStream().Name(name).Do(ctx)
	if err != nil {
		return []types.DataStream{}, fmt.Errorf("cannot GET datastream: %w", err)
	}

	return resp.DataStreams, nil
}

// ApmDocCount is used to unmarshal response from ES|QL query in ApmDocCount().
type ApmDocCount struct {
	Count      int
	Datastream string
}

// APMDataStreamsDocCount is an easy to assert on format reporting doc count for
// APM data streams.
type APMDataStreamsDocCount map[string]int

func (c *Client) ApmDocCount(ctx context.Context) (APMDataStreamsDocCount, error) {
	q := `FROM traces-apm*,apm-*,traces-*.otel-*,logs-apm*,apm-*,logs-*.otel-*,metrics-apm*,apm-*,metrics-*.otel-*
| EVAL datastream = CONCAT(data_stream.type, "-", data_stream.dataset, "-", data_stream.namespace)
| STATS count = COUNT(*) BY datastream
| SORT count DESC`

	qry := c.es.Esql.Query().Query(q)
	resp, err := query.Helper[ApmDocCount](ctx, qry)
	if err != nil {
		var eserr *types.ElasticsearchError
		// suppress this error as it only indicates no data is available yet.
		expected := `Found 1 problem
line 1:1: Unknown index [traces-apm*,apm-*,traces-*.otel-*,logs-apm*,apm-*,logs-*.otel-*,metrics-apm*,apm-*,metrics-*.otel-*]`
		if errors.As(err, &eserr) &&
			eserr.ErrorCause.Reason != nil &&
			*eserr.ErrorCause.Reason == expected {
			return APMDataStreamsDocCount{}, nil
		}

		return APMDataStreamsDocCount{}, fmt.Errorf("cannot retrieve APM doc count: %w", err)
	}

	res := APMDataStreamsDocCount{}
	for _, dc := range resp {
		res[dc.Datastream] = dc.Count
	}

	return res, nil
}

// GetESErrorLogs retrieves Elasticsearch error logs.
// The search query is on the Index used by Elasticsearch monitoring to store logs.
func (c *Client) GetESErrorLogs(ctx context.Context) (*search.Response, error) {
	res, err := c.es.Search().
		Index("elastic-cloud-logs-8").
		Request(&search.Request{
			Query: &types.Query{
				Bool: &types.BoolQuery{
					Must: []types.Query{
						{
							Match: map[string]types.MatchQuery{
								"service.type": {Query: "elasticsearch"},
							},
						},
						{
							Match: map[string]types.MatchQuery{
								"log.level": {Query: "ERROR"},
							},
						},
					},
				},
			},
		}).Do(ctx)
	if err != nil {
		return search.NewResponse(), fmt.Errorf("cannot run search query: %w", err)
	}

	return res, nil
}
