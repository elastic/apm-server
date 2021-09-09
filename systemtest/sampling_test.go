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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

func TestKeepUnsampled(t *testing.T) {
	for _, keepUnsampled := range []bool{false, true} {
		t.Run(fmt.Sprint(keepUnsampled), func(t *testing.T) {
			systemtest.CleanupElasticsearch(t)
			srv := apmservertest.NewUnstartedServer(t)
			srv.Config.Sampling = &apmservertest.SamplingConfig{
				KeepUnsampled: keepUnsampled,
			}
			err := srv.Start()
			require.NoError(t, err)

			// Send one unsampled transaction, and one sampled transaction.
			transactionType := "TestKeepUnsampled"
			tracer := srv.Tracer()
			tracer.StartTransaction("sampled", transactionType).End()
			tracer.SetSampler(apm.NewRatioSampler(0))
			tracer.StartTransaction("unsampled", transactionType).End()
			tracer.Flush(nil)

			expectedTransactionDocs := 1
			if keepUnsampled {
				expectedTransactionDocs++
			}

			result := systemtest.Elasticsearch.ExpectMinDocs(t, expectedTransactionDocs, "apm-*", estest.TermQuery{
				Field: "transaction.type",
				Value: transactionType,
			})
			assert.Len(t, result.Hits.Hits, expectedTransactionDocs)
		})
	}
}

func TestKeepUnsampledWarning(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Sampling = &apmservertest.SamplingConfig{KeepUnsampled: false}
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		Transactions: &apmservertest.TransactionAggregationConfig{Enabled: false},
	}
	require.NoError(t, srv.Start())
	require.NoError(t, srv.Close())

	var messages []string
	for _, log := range srv.Logs.All() {
		messages = append(messages, log.Message)
	}
	assert.Contains(t, messages, ""+
		"apm-server.sampling.keep_unsampled and apm-server.aggregation.transactions.enabled are both false, "+
		"which will lead to incorrect metrics being reported in the APM UI",
	)
}

func TestTailSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	cleanupFleet(t, systemtest.Fleet)
	integrationPackage := getAPMIntegrationPackage(t, systemtest.Fleet)
	err := systemtest.Fleet.InstallPackage(integrationPackage.Name, integrationPackage.Version)
	require.NoError(t, err)

	srv1 := apmservertest.NewUnstartedServer(t)
	srv1.Config.DataStreams = &apmservertest.DataStreamsConfig{Enabled: true}
	srv1.Config.Sampling = &apmservertest.SamplingConfig{
		Tail: &apmservertest.TailSamplingConfig{
			Enabled:  true,
			Interval: time.Second,
			Policies: []apmservertest.TailSamplingPolicy{{SampleRate: 0.5}},
		},
	}
	srv1.Config.Monitoring = newFastMonitoringConfig()
	require.NoError(t, srv1.Start())

	srv2 := apmservertest.NewUnstartedServer(t)
	srv2.Config.DataStreams = &apmservertest.DataStreamsConfig{Enabled: true}
	srv2.Config.Sampling = srv1.Config.Sampling
	require.NoError(t, srv2.Start())

	const total = 200
	const expected = 100 // 50%

	tracer1 := srv1.Tracer()
	tracer2 := srv2.Tracer()
	for i := 0; i < total; i++ {
		parent := tracer1.StartTransaction("GET /", "parent")
		parent.Duration = time.Second * time.Duration(i+1)
		child := tracer2.StartTransactionOptions("GET /", "child", apm.TransactionOptions{
			TraceContext: parent.TraceContext(),
		})
		child.Duration = 500 * time.Millisecond * time.Duration(i+1)
		child.End()
		parent.End()
	}
	tracer1.Flush(nil)
	tracer2.Flush(nil)

	// Flush the data stream while the test is running, as we have no
	// control over the settings for the sampled traces index template.
	refreshPeriodically(t, 250*time.Millisecond, "traces-apm.sampled-*")

	for _, transactionType := range []string{"parent", "child"} {
		var result estest.SearchResult
		t.Logf("waiting for %d %q transactions", expected, transactionType)
		_, err := systemtest.Elasticsearch.Search("traces-*").WithQuery(estest.TermQuery{
			Field: "transaction.type",
			Value: transactionType,
		}).WithSize(total).Do(context.Background(), &result,
			estest.WithCondition(result.Hits.MinHitsCondition(expected)),
		)
		require.NoError(t, err)
		assert.Len(t, result.Hits.Hits, expected)
	}

	// Make sure apm-server.sampling.tail metrics are published. Metric values are unit tested.
	doc := getBeatsMonitoringStats(t, srv1, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.sampling.tail").Exists())

	// Check tail-sampling config is reported in telemetry.
	var state struct {
		APMServer struct {
			Sampling struct {
				Tail struct {
					Enabled  bool
					Policies int
				}
			}
		} `mapstructure:"apm-server"`
	}
	getBeatsMonitoringState(t, srv1, &state)
	assert.True(t, state.APMServer.Sampling.Tail.Enabled)
	assert.Equal(t, 1, state.APMServer.Sampling.Tail.Policies)
}

func TestTailSamplingUnlicensed(t *testing.T) {
	// Start an ephemeral Elasticsearch container with a Basic license to
	// test that tail-based sampling requires a platinum or trial license.
	es, err := systemtest.NewUnstartedElasticsearchContainer()
	require.NoError(t, err)
	es.Env["xpack.license.self_generated.type"] = "basic"
	require.NoError(t, es.Start())
	defer es.Close()

	// Data streams are required for tail-based sampling, but since we're using
	// an ephemeral Elasticsearch container it's not straightforward to install
	// the integration package. We won't be indexing anything, so just don't wait
	// for the integration package to be installed in this test.
	waitForIntegration := false
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Output.Elasticsearch.Hosts = []string{es.Addr}
	srv.Config.DataStreams = &apmservertest.DataStreamsConfig{
		Enabled:            true,
		WaitForIntegration: &waitForIntegration,
	}
	srv.Config.Sampling = &apmservertest.SamplingConfig{
		Tail: &apmservertest.TailSamplingConfig{
			Enabled:  true,
			Interval: time.Second,
			Policies: []apmservertest.TailSamplingPolicy{{SampleRate: 0.5}},
		},
	}
	require.NoError(t, srv.Start())

	// Send some transactions to trigger an indexing attempt.
	tracer := srv.Tracer()
	for i := 0; i < 100; i++ {
		tx := tracer.StartTransaction("GET /", "parent")
		tx.Duration = time.Second * time.Duration(i+1)
		tx.End()
	}
	tracer.Flush(nil)

	timeout := time.After(time.Minute)
	logs := srv.Logs.Iterator()
	var done bool
	for !done {
		select {
		case entry := <-logs.C():
			done = strings.Contains(entry.Message, "invalid license")
		case <-timeout:
			t.Fatal("timed out waiting for log message")
		}
	}

	// Due to the failing license check, APM Server will refuse to index anything.
	var result estest.SearchResult
	_, err = es.Client.Search("traces-apm*").Do(context.Background(), &result)
	assert.NoError(t, err)
	assert.Empty(t, result.Hits.Hits)

	// The server will wait for the enqueued events to be published before
	// shutting down gracefully, so shutdown forcefully.
	srv.Kill()
}

func refreshPeriodically(t *testing.T, interval time.Duration, index ...string) {
	g, ctx := errgroup.WithContext(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, g.Wait())
	})
	g.Go(func() error {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		allowNoIndices := true
		ignoreUnavailable := true
		request := esapi.IndicesRefreshRequest{
			Index:             index,
			AllowNoIndices:    &allowNoIndices,
			IgnoreUnavailable: &ignoreUnavailable,
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
			if _, err := systemtest.Elasticsearch.Do(ctx, &request, nil); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
	})
}
