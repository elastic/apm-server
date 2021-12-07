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
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestDropUnsampled(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	err := srv.Start()
	require.NoError(t, err)
	defaultMetadataFilter := srv.EventMetadataFilter

	// Send:
	// - one sampled transaction (should be stored)
	// - one unsampled RUM transaction (should be be stored)
	// - one unsampled backend transaction (should be dropped)
	transactionType := "TestDropUnsampled"
	timestamp := time.Unix(0, 0)
	sendTransaction := func(sampled bool, agentName string) {
		srv.EventMetadataFilter = apmservertest.EventMetadataFilterFunc(func(m *apmservertest.EventMetadata) {
			defaultMetadataFilter.FilterEventMetadata(m)
			m.Service.Agent.Name = agentName
		})
		tracer := srv.Tracer()
		defer tracer.Flush(nil)
		transactionName := "unsampled"
		if sampled {
			transactionName = "sampled"
		}
		timestamp = timestamp.Add(time.Second)
		tx := tracer.StartTransactionOptions(transactionName, transactionType, apm.TransactionOptions{
			Start: timestamp,
			TraceContext: apm.TraceContext{
				Trace:   apm.TraceID{1},
				Options: apm.TraceOptions(0).WithRecorded(sampled),
			},
			TransactionID: apm.SpanID{2},
		})
		tx.Duration = time.Second
		tx.End()
	}
	sendTransaction(false, "backend")
	sendTransaction(false, "rum-js")
	sendTransaction(true, "backend")

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "traces-apm*", estest.TermQuery{
		Field: "transaction.type",
		Value: transactionType,
	})
	assert.Len(t, result.Hits.Hits, 2)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)

	doc := getBeatsMonitoringStats(t, srv, nil)
	transactionsDropped := gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.sampling.transactions_dropped")
	assert.Equal(t, int64(1), transactionsDropped.Int())
}

func TestTailSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	apmIntegration1 := newAPMIntegration(t, map[string]interface{}{
		"tail_sampling_interval": "1s",
		"tail_sampling_policies": []map[string]interface{}{{"sample_rate": 0.5}},
	})

	apmIntegration2 := newAPMIntegration(t, map[string]interface{}{
		"tail_sampling_interval": "1s",
		"tail_sampling_policies": []map[string]interface{}{{"sample_rate": 0.5}},
	})

	const total = 200
	const expected = 100 // 50%

	tracer1 := apmIntegration1.Tracer
	tracer2 := apmIntegration2.Tracer
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
		assert.Equal(t, expected, len(result.Hits.Hits), transactionType)
	}

	// Make sure apm-server.sampling.tail metrics are published. Metric values are unit tested.
	doc := apmIntegration1.getBeatsMonitoringStats(t, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "apm-server.sampling.tail").Exists())

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
	apmIntegration1.getBeatsMonitoringState(t, &state)
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
	srv.Config.WaitForIntegration = &waitForIntegration
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
