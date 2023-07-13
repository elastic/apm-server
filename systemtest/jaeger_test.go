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
	"net/url"
	"os"
	"testing"

	jaegermodel "github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestJaeger(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	require.NoError(t, srv.Start())

	for _, name := range []string{"batch_0", "batch_1"} {
		t.Run(name, func(t *testing.T) {
			systemtest.CleanupElasticsearch(t)
			hits := sendJaegerBatch(t, srv, "../testdata/jaeger/"+name+".json", grpc.WithInsecure())
			approvaltest.ApproveEvents(t, t.Name(), hits)
		})
	}

	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.GreaterOrEqual(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.jaeger.grpc.collect.request.count").Int(), int64(1))
}

func TestJaegerMuxedTLS(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	srv.Config.TLS = &apmservertest.TLSConfig{ClientAuthentication: "required"}
	require.NoError(t, srv.StartTLS())
	sendJaegerBatch(t, srv,
		"../testdata/jaeger/batch_0.json",
		grpc.WithTransportCredentials(credentials.NewTLS(srv.TLS)),
	)
}

func sendJaegerBatch(
	t *testing.T, srv *apmservertest.Server,
	filename string,
	dialOptions ...grpc.DialOption,
) []espoll.SearchHit {
	conn, err := grpc.Dial(serverAddr(srv), dialOptions...)
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	request, err := decodeJaegerPostSpansRequest(filename)
	require.NoError(t, err)
	_, err = client.PostSpans(context.Background(), request)
	require.NoError(t, err)

	expected := len(request.Batch.Spans)
	for _, span := range request.Batch.Spans {
		expected += len(span.GetLogs())
	}

	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, expected, "traces-apm*,logs-apm*", nil)
	assert.Equal(t, expected, result.Hits.Total.Value)
	return result.Hits.Hits
}

func TestJaegerSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	conn, err := grpc.Dial(serverAddr(srv), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewSamplingManagerClient(conn)
	_, err = client.GetSamplingStrategy(
		context.Background(), &api_v2.SamplingStrategyParameters{ServiceName: "missing"},
	)
	require.Error(t, err)
	assert.Regexp(t, "no sampling rate available", err.Error())
}

func TestJaegerAuth(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.AgentAuth.SecretToken = "secret"
	require.NoError(t, srv.Start())

	conn, err := grpc.Dial(serverAddr(srv), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	request, err := decodeJaegerPostSpansRequest("../testdata/jaeger/batch_0.json")
	require.NoError(t, err)

	// Attempt to send spans without the auth tag -- this should fail.
	_, err = client.PostSpans(context.Background(), request)
	require.Error(t, err)
	status := status.Convert(err)
	assert.Equal(t, codes.Unauthenticated, status.Code())

	// Now with the auth tag -- this should succeed.
	request.Batch.Process.Tags = append(request.Batch.Process.Tags, jaegermodel.KeyValue{
		Key:   "elastic-apm-auth",
		VType: jaegermodel.ValueType_STRING,
		VStr:  "Bearer secret",
	})
	_, err = client.PostSpans(context.Background(), request)
	require.NoError(t, err)

	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*", espoll.BoolQuery{Filter: []interface{}{
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	}})
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

func serverAddr(srv *apmservertest.Server) string {
	url, _ := url.Parse(srv.URL)
	return url.Host
}
