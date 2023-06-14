// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

// This file is mandatory as otherwise the apm-server.test binary is not generated correctly.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2/apmtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/paths"

	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/elasticsearch"
)

func TestMonitoring(t *testing.T) {
	// samplingMonitoringRegistry will be nil, as under normal circumstances
	// we rely on apm-server/sampling to create the registry.
	samplingMonitoringRegistry = monitoring.NewRegistry()

	home := t.TempDir()
	err := paths.InitPaths(&paths.Path{Home: home})
	require.NoError(t, err)
	defer closeBadger() // close badger.DB so data dir can be deleted on Windows

	cfg := config.DefaultConfig()
	cfg.Sampling.Tail.Enabled = true
	cfg.Sampling.Tail.Policies = []config.TailSamplingPolicy{{SampleRate: 0.1}}
	// MaxTransactionGroups, MaxServices and MaxGroups are configured based on memory limit.
	// Overriding here to avoid validation errors.
	cfg.Aggregation.Transactions.MaxTransactionGroups = 10000
	cfg.Aggregation.Transactions.MaxServices = 10000
	cfg.Aggregation.ServiceTransactions.MaxGroups = 10000

	// Wrap & run the server twice, to ensure metric registration does not panic.
	runServerError := errors.New("runServer")
	for i := 0; i < 2; i++ {
		var aggregationMonitoringSnapshot, tailSamplingMonitoringSnapshot monitoring.FlatSnapshot
		serverParams, runServer, err := wrapServer(beater.ServerParams{
			Config:                 cfg,
			Logger:                 logp.NewLogger(""),
			Tracer:                 apmtest.DiscardTracer,
			BatchProcessor:         modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error { return nil }),
			Namespace:              "default",
			NewElasticsearchClient: elasticsearch.NewClient,
		}, func(ctx context.Context, args beater.ServerParams) error {
			aggregationMonitoringSnapshot = monitoring.CollectFlatSnapshot(aggregationMonitoringRegistry, monitoring.Full, false)
			tailSamplingMonitoringSnapshot = monitoring.CollectFlatSnapshot(samplingMonitoringRegistry, monitoring.Full, false)
			return runServerError
		})
		require.NoError(t, err)

		err = runServer(context.Background(), serverParams)
		assert.Equal(t, runServerError, err)
		assert.NotEqual(t, monitoring.MakeFlatSnapshot(), aggregationMonitoringSnapshot)
		assert.NotEqual(t, monitoring.MakeFlatSnapshot(), tailSamplingMonitoringSnapshot)
	}
}

func TestQueryElasticsearchClusterName(t *testing.T) {
	const clusterName = "test-cluster-123"
	cases := []struct {
		name,
		expectedResult,
		body string
		throwErr       bool
		expectedLogMsg string
	}{
		{
			name:           "valid_JSON_response_with_cluster_name",
			expectedResult: clusterName,
			body:           fmt.Sprintf(`{"cluster_name":"%s"}`, clusterName),
		}, {
			name:           "valid_JSON_response_without_cluster_name",
			expectedResult: "",
			body:           `{"anything":"42"}`,
			expectedLogMsg: "failed to parse Elasticsearch JSON response",
		}, {
			name:           "invalid_JSON_response",
			expectedResult: "",
			body:           "::error::",
			expectedLogMsg: "failed to parse Elasticsearch JSON response",
		}, {
			name:           "server_unresponsive",
			expectedResult: "",
			body:           "",
			throwErr:       true,
			expectedLogMsg: "failed to fetch cluster name from Elasticsearch",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			logger := logp.NewLogger("go_tests_apm_server", zap.Hooks(detectMessageInLog(t, tc.expectedLogMsg)))
			var client *elasticsearch.Client
			if tc.throwErr {
				conf := elasticsearch.DefaultConfig()
				// This is intentionally going to fail, no need to retry.
				conf.MaxRetries = 1
				// We cannot disable retries so the first retry is going to block and kickin
				// the backoff period.
				// Set the backoff init to 0 to disable the backoff.
				conf.Backoff.Init = 0
				client, _ = elasticsearch.NewClientParams(elasticsearch.ClientParams{
					Config:    conf,
					Transport: errTransport{},
				})
			} else {
				client = newMockElasticsearchClient(t, tc.body)
			}
			result, err := queryElasticsearchClusterName(client, logger)
			if tc.throwErr {
				// Even when the server does not reply, we don't want to return an error to the caller
				require.Nil(t, err)
				assert.Empty(t, result)
				return
			}
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func detectMessageInLog(t *testing.T, contained string) func(zapcore.Entry) error {
	return func(entry zapcore.Entry) error {
		assert.Equal(t, entry.Level, logp.WarnLevel)
		if !assert.Contains(t, entry.Message, contained) {
			t.Fatalf("didn't find '%s' in log Message field", contained)
		}
		return nil
	}
}

func newMockElasticsearchClient(t testing.TB, body string) *elasticsearch.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client, err := elasticsearch.NewClient(&elasticsearch.Config{Hosts: []string{srv.URL}})
	require.NoError(t, err)
	return client
}

type errTransport struct{}

func (errTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("connection closed")
}
