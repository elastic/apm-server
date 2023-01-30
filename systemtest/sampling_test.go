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
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.elastic.co/apm/v2"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestDropUnsampled(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	srv.Config.RUM = &apmservertest.RUMConfig{
		Enabled: true,
	}
	srv.Config.AgentAuth.Anonymous = &apmservertest.AnonymousAuthConfig{
		Enabled: true,
	}
	err := srv.Start()
	require.NoError(t, err)

	// Sampled transaction (should be stored)
	tracer := srv.Tracer()
	tx := tracer.StartTransactionOptions("sampled", "TestDropUnsampled", apm.TransactionOptions{
		Start: time.Unix(0, 0), // set timestamp for sorting purposes
	})
	tx.Duration = time.Second
	tx.End()
	tracer.Flush(nil)

	// Unsampled backend transaction (should be dropped)
	systemtest.SendBackendEventsLiteral(t, srv.URL, `
{"metadata":{"service":{"name":"allowed","version":"1.0.0","agent":{"name":"backend","version":"0.0.0"}}}}
{"transaction":{"sampled":false,"trace_id":"xyz","id":"yz","type":"TestDropUnsampled","duration":0,"span_count":{"started":1},"context":{"service":{"name":"allowed"}}}}`[1:])

	// Unsampled RUM transaction (should be stored)
	systemtest.SendRUMEventsLiteral(t, srv.URL, `
{"metadata":{"service":{"name":"allowed","version":"1.0.0","agent":{"name":"rum-js","version":"0.0.0"}}}}
{"transaction":{"sampled":false,"trace_id":"x","id":"y","type":"TestDropUnsampled","duration":0,"span_count":{"started":1},"context":{"service":{"name":"allowed"}}}}`[1:])

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "traces-apm*", estest.TermQuery{
		Field: "transaction.type",
		Value: "TestDropUnsampled",
	})
	assert.Len(t, result.Hits.Hits, 2)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
		// Ignore dynamically generated trace/transaction ID
		"trace.id", "transaction.id",
	)

	doc := getBeatsMonitoringStats(t, srv, nil)
	transactionsDropped := gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.sampling.transactions_dropped")
	assert.Equal(t, int64(1), transactionsDropped.Int())
}

func TestTailSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	var wg sync.WaitGroup
	wg.Add(2)
	var apmIntegration1 apmIntegration
	var apmIntegration2 apmIntegration

	go func() {
		defer wg.Done()
		apmIntegration1 = newAPMIntegration(t, map[string]interface{}{
			"tail_sampling_enabled":  true,
			"tail_sampling_interval": "1s",
			"tail_sampling_policies": []map[string]interface{}{{"sample_rate": 0.5}},
		})
	}()

	go func() {
		defer wg.Done()
		apmIntegration2 = newAPMIntegration(t, map[string]interface{}{
			"tail_sampling_enabled":  true,
			"tail_sampling_interval": "1s",
			"tail_sampling_policies": []map[string]interface{}{{"sample_rate": 0.5}},
		})
	}()

	wg.Wait()

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
