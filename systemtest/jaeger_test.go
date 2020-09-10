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

package systemtest_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestJaegerGRPC(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Jaeger = &apmservertest.JaegerConfig{
		GRPCEnabled: true,
		GRPCHost:    "localhost:0",
	}
	err := srv.Start()
	require.NoError(t, err)

	conn, err := grpc.Dial(srv.JaegerGRPCAddr, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	request, err := decodeJaegerPostSpansRequest("../testdata/jaeger/batch_0.json")
	require.NoError(t, err)
	_, err = client.PostSpans(context.Background(), request)
	require.NoError(t, err)

	var result estest.SearchResult
	_, err = systemtest.Elasticsearch.Search("apm-*").WithQuery(estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{
				Field: "processor.event",
				Value: "transaction",
			},
		},
	}).Do(context.Background(), &result, estest.WithCondition(result.Hits.NonEmptyCondition()))
	require.NoError(t, err)

	// TODO(axw) check document contents. We currently do this in beater/jaeger.
}

func TestJaegerGRPCSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Jaeger = &apmservertest.JaegerConfig{
		GRPCEnabled: true,
		GRPCHost:    "localhost:0",
	}
	err := srv.Start()
	require.NoError(t, err)

	conn, err := grpc.Dial(srv.JaegerGRPCAddr, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewSamplingManagerClient(conn)
	_, err = client.GetSamplingStrategy(
		context.Background(), &api_v2.SamplingStrategyParameters{ServiceName: "missing"},
	)
	require.Error(t, err)
	assert.Regexp(t, "no sampling rate available", err.Error())
}

func decodeJaegerPostSpansRequest(filename string) (*api_v2.PostSpansRequest, error) {
	var request api_v2.PostSpansRequest
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return &request, json.NewDecoder(f).Decode(&request)
}
