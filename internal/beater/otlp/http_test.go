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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"golang.org/x/sync/semaphore"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/api"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestConsumeTracesHTTP(t *testing.T) {
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal trace to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operation_name")

	tracesRequest := ptraceotlp.NewExportRequestFromTraces(traces)
	request, err := tracesRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/traces", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())
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
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal metric to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	metrics := pmetric.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("metric_type")
	metric.SetEmptySummary()
	metric.Summary().DataPoints().AppendEmpty()

	metricsRequest := pmetricotlp.NewExportRequestFromMetrics(metrics)
	request, err := metricsRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/metrics", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.http.metrics").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		"consumer.unsupported_dropped": int64(0),

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
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr := newHTTPServer(t, batchProcessor)

	// Send a minimal log record to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	logsRequest := plogotlp.NewExportRequestFromLogs(logs)
	request, err := logsRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/logs", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())
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

func newHTTPServer(t *testing.T, batchProcessor modelpb.BatchProcessor) string {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	cfg := &config.Config{}
	auth, _ := auth.NewAuthenticator(cfg.AgentAuth, logptest.NewTestingLogger(t, ""))
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	router, err := api.NewMux(
		cfg,
		batchProcessor,
		auth,
		agentcfg.NewDirectFetcher(nil),
		ratelimitStore,
		nil,
		func() bool { return true },
		semaphore.NewWeighted(1),
		logp.NewLogger(""),
	)
	require.NoError(t, err)
	srv := http.Server{Handler: router}
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})
	go srv.Serve(lis)
	return lis.Addr().String()
}
