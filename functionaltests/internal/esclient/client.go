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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/esql/query"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/rollover"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v8/typedapi/security/createapikey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type Client struct {
	es *elasticsearch.TypedClient
}

// New returns a new Client for accessing Elasticsearch.
func New(esURL, username, password string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	es, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{esURL},
		Username:  username,
		Password:  password,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch client: %w", err)
	}
	return &Client{es: es}, nil
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

// CreateIngestPipeline creates a new pipeline with the provided name and processors.
//
// Refer to https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest.html.
func (c *Client) CreateIngestPipeline(ctx context.Context, pipeline string, processors []types.ProcessorContainer) error {
	_, err := c.es.Ingest.PutPipeline(pipeline).
		Request(&putpipeline.Request{Processors: processors}).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("error creating ingest pipeline for %s: %w", pipeline, err)
	}
	return nil
}

// PerformManualRollover performs an immediate manual rollover for the specified data stream.
//
// Refer to https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-rollover-index.html.
func (c *Client) PerformManualRollover(ctx context.Context, dataStream string) error {
	_, err := c.es.Indices.Rollover(dataStream).Request(&rollover.Request{}).Do(ctx)
	if err != nil {
		return fmt.Errorf("error performing manual rollover for %s: %w", dataStream, err)
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

// DocCount is used to unmarshal response from ES|QL query.
type DocCount struct {
	DataStream string
	Count      int
}

// DataStreamsDocCount is an easy to assert on format reporting document count
// for data streams.
type DataStreamsDocCount map[string]int

func (c *Client) APMDocCount(ctx context.Context) (DataStreamsDocCount, error) {
	q := `FROM traces-apm*,apm-*,traces-*.otel-*,logs-apm*,apm-*,logs-*.otel-*,metrics-apm*,apm-*,metrics-*.otel-*
| EVAL datastream = CONCAT(data_stream.type, "-", data_stream.dataset, "-", data_stream.namespace)
| STATS count = COUNT(*) BY datastream
| SORT count DESC`

	qry := c.es.Esql.Query().Query(q)
	resp, err := query.Helper[DocCount](ctx, qry)
	if err != nil {
		var eserr *types.ElasticsearchError
		// suppress this error as it only indicates no data is available yet.
		expected := `Found 1 problem
line 1:1: Unknown index [traces-apm*,apm-*,traces-*.otel-*,logs-apm*,apm-*,logs-*.otel-*,metrics-apm*,apm-*,metrics-*.otel-*]`
		if errors.As(err, &eserr) &&
			eserr.ErrorCause.Reason != nil &&
			*eserr.ErrorCause.Reason == expected {
			return DataStreamsDocCount{}, nil
		}

		return DataStreamsDocCount{}, fmt.Errorf("cannot retrieve APM doc count: %w", err)
	}

	res := DataStreamsDocCount{}
	for _, dc := range resp {
		res[dc.DataStream] = dc.Count
	}

	return res, nil
}

// GetESErrorLogs retrieves Elasticsearch error logs.
// The search query is on the Index used by Elasticsearch monitoring to store logs.
// exclude allows to pass in must_not clauses to be applied to the query to filter
// the returned results.
func (c *Client) GetESErrorLogs(ctx context.Context, exclude ...types.Query) (*search.Response, error) {
	// There is an issue in ES: https://github.com/elastic/elasticsearch/issues/125445,
	// that is causing deprecation logger bulk write failures.
	// The error itself is harmless and irrelevant to APM, so we can ignore it.
	// TODO: Remove this query once the above issue is fixed.
	exclude = append(exclude, types.Query{
		Match: map[string]types.MatchQuery{
			"message": {Query: "Bulk write of deprecation logs encountered some failures"},
		}})
	res, err := c.es.Search().
		Index("elastic-cloud-logs-*").
		Request(&search.Request{
			Query: &types.Query{
				Bool: &types.BoolQuery{
					Must: []types.Query{
						{
							Match: map[string]types.MatchQuery{
								"service.type": {Query: "elasticsearch"},
							},
						},
					},
					Should: []types.Query{
						{
							Match: map[string]types.MatchQuery{
								"log.level": {Query: "ERROR"},
							},
						},
						{
							Match: map[string]types.MatchQuery{
								"log.level": {Query: "error"},
							},
						},
					},
					MustNot: exclude,
				},
			},
		}).Do(ctx)
	if err != nil {
		return search.NewResponse(), fmt.Errorf("cannot run search query: %w", err)
	}

	return res, nil
}

// GetAPMErrorLogs retrieves Elasticsearch error logs.
// The search query is on the Index used by Elasticsearch monitoring to store logs.
// exclude allows to pass in must_not clauses to be applied to the query to filter
// the returned results.
func (c *Client) GetAPMErrorLogs(ctx context.Context, exclude []types.Query) (*search.Response, error) {
	size := 100
	res, err := c.es.Search().
		Index("elastic-cloud-logs-*").
		Request(&search.Request{
			Size: &size,
			Query: &types.Query{
				Bool: &types.BoolQuery{
					Must: []types.Query{
						{
							Match: map[string]types.MatchQuery{
								"service.name": {Query: "apm-server"},
							},
						},
					},
					Should: []types.Query{
						{
							Match: map[string]types.MatchQuery{
								"log.level": {Query: "ERROR"},
							},
						},
						{
							Match: map[string]types.MatchQuery{
								"log.level": {Query: "error"},
							},
						},
					},
					MustNot: exclude,
				},
			},
		}).Do(ctx)
	if err != nil {
		return search.NewResponse(), fmt.Errorf("cannot run search query: %w", err)
	}

	return res, nil
}
