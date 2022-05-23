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

package otlp_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/otlpgrpc"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestConsumeTracesHTTP(t *testing.T) {
	var batches []model.Batch
	var reportError error
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal trace to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	traces := pdata.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operation_name")

	tracesRequest := otlpgrpc.NewTracesRequest()
	tracesRequest.SetTraces(traces)
	request, err := tracesRequest.Marshal()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/traces", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	_, err = client.Do(req)
	assert.NoError(t, err)
	require.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.http.traces").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		"request.count":                int64(1),
		"response.count":               int64(1),
		"response.errors.count":        int64(0),
		"response.valid.count":         int64(1),
		"response.errors.ratelimit":    int64(0),
		"response.errors.timeout":      int64(0),
		"response.errors.unauthorized": int64(0),
	}, actual)
}

func TestConsumeMetricsHTTP(t *testing.T) {
	var reportError error
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal metric to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	metrics := pdata.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("metric_type")
	metric.SetDataType(pdata.MetricDataTypeSummary)
	metric.Summary().DataPoints().AppendEmpty()

	metricsRequest := otlpgrpc.NewMetricsRequest()
	metricsRequest.SetMetrics(metrics)
	request, err := metricsRequest.Marshal()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/metrics", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	_, err = client.Do(req)
	assert.NoError(t, err)

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.http.metrics").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		// In the request we send above,
		// the metrics do not have a type and so
		// we treat them as unsupported metrics.
		"consumer.unsupported_dropped": int64(1),

		"request.count":                int64(1),
		"response.count":               int64(1),
		"response.errors.count":        int64(0),
		"response.valid.count":         int64(1),
		"response.errors.ratelimit":    int64(0),
		"response.errors.timeout":      int64(0),
		"response.errors.unauthorized": int64(0),
	}, actual)
}

func TestConsumeLogsHTTP(t *testing.T) {
	var batches []model.Batch
	var reportError error
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal log record to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	logs := pdata.NewLogs()
	logRecord := logs.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.SetName("log_name")

	logsRequest := otlpgrpc.NewLogsRequest()
	logsRequest.SetLogs(logs)
	request, err := logsRequest.Marshal()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/logs", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	_, err = client.Do(req)
	assert.NoError(t, err)
	require.Len(t, batches, 1)

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.http.logs").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		"request.count":                int64(1),
		"response.count":               int64(1),
		"response.errors.count":        int64(0),
		"response.valid.count":         int64(1),
		"response.errors.ratelimit":    int64(0),
		"response.errors.timeout":      int64(0),
		"response.errors.unauthorized": int64(0),
	}, actual)
}

func newHTTPServer(t *testing.T, batchProcessor model.BatchProcessor) string {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	handlers, err := otlp.NewHTTPHandlers(batchProcessor)
	require.NoError(t, err)
	cfg := &config.Config{}
	auth, _ := auth.NewAuthenticator(cfg.AgentAuth)
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	router, err := api.NewMux(beat.Info{Version: "1.2.3"}, cfg, batchProcessor, auth, agentcfg.NewFetcher(cfg), ratelimitStore, nil, handlers, false, func() bool { return true })
	require.NoError(t, err)
	srv := http.Server{Handler: router}
	go srv.Serve(lis)
	return lis.Addr().String()
}
