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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
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

func TestTailSampling(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	// Create the apm-sampled-traces index for the two servers to coordinate.
	_, err := systemtest.Elasticsearch.Do(context.Background(), &esapi.IndicesCreateRequest{
		Index: "apm-sampled-traces",
		Body: strings.NewReader(`{
  "mappings": {
    "properties": {
      "event.ingested": {"type": "date"},
      "observer": {
        "properties": {
          "id": {"type": "keyword"}
        }
      },
      "trace": {
        "properties": {
          "id": {"type": "keyword"}
        }
      }
    }
  }
}`),
	}, nil)
	require.NoError(t, err)

	srv1 := apmservertest.NewUnstartedServer(t)
	srv1.Config.Sampling = &apmservertest.SamplingConfig{
		Tail: &apmservertest.TailSamplingConfig{
			Enabled:  true,
			Interval: time.Second,
			Policies: []apmservertest.TailSamplingPolicy{{SampleRate: 0.5}},
		},
	}
	require.NoError(t, srv1.Start())

	srv2 := apmservertest.NewUnstartedServer(t)
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

	for _, transactionType := range []string{"parent", "child"} {
		var result estest.SearchResult
		t.Logf("waiting for %d %q transactions", expected, transactionType)
		_, err := systemtest.Elasticsearch.Search("apm-*").WithQuery(estest.TermQuery{
			Field: "transaction.type",
			Value: transactionType,
		}).WithSize(total).Do(context.Background(), &result,
			estest.WithCondition(result.Hits.MinHitsCondition(expected)),
		)
		require.NoError(t, err)
		assert.Len(t, result.Hits.Hits, expected)
	}
}

func TestTailSamplingUnlicensed(t *testing.T) {
	// Start an ephemeral Elasticsearch container with a Basic license to
	// test that tail-based sampling requires a platinum or trial license.
	es, err := systemtest.NewUnstartedElasticsearchContainer()
	require.NoError(t, err)
	es.Env["xpack.license.self_generated.type"] = "basic"
	require.NoError(t, es.Start())
	defer es.Close()

	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Output.Elasticsearch.Hosts = []string{es.Addr}
	srv.Config.Sampling = &apmservertest.SamplingConfig{
		Tail: &apmservertest.TailSamplingConfig{
			Enabled:  true,
			Interval: time.Second,
			Policies: []apmservertest.TailSamplingPolicy{{SampleRate: 0.5}},
		},
	}
	require.NoError(t, srv.Start())

	timeout := time.After(time.Minute)
	logs := srv.Logs.Iterator()
	for {
		select {
		case entry := <-logs.C():
			if strings.Contains(entry.Message, "invalid license") {
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for log message")
		}
	}

	// Due to the failing license check, APM Server will refuse to index anything.
	var result estest.SearchResult
	_, err = es.Client.Search("apm-*").Do(context.Background(), &result)
	assert.NoError(t, err)
	assert.Empty(t, result.Hits.Hits)
}
