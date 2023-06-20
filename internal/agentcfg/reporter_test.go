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

package agentcfg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestReportFetch(t *testing.T) {
	interval := 10 * time.Millisecond
	receivedc := make(chan struct{})
	defer close(receivedc)
	bp := &batchProcessor{receivedc: receivedc}
	r := NewReporter(fauxFetcher{}, bp, interval)

	var g errgroup.Group
	ctx, cancel := context.WithCancel(context.Background())
	g.Go(func() error { return r.Run(ctx) })

	query1 := Query{
		Service: Service{Name: "webapp", Environment: "production"},
		Etag:    "abc123",
	}
	query2 := Query{
		Etag:                 "def456",
		MarkAsAppliedByAgent: true,
	}
	query3 := Query{
		Etag: "old-etag",
	}
	query4 := Query{
		Etag:    "-",
		Service: Service{Name: "non_matching"},
	}
	r.Fetch(ctx, query1)
	r.Fetch(ctx, query2)
	r.Fetch(ctx, query3)
	r.Fetch(ctx, query4)
	<-receivedc
	<-receivedc
	<-receivedc

	// cancel the context to stop processing
	cancel()
	g.Wait()

	for i, received := range bp.received {
		// Assert the timestamp is not empty and set the timestamp to an empty
		// value so we can assert equality in the list contents.
		assert.NotZero(t, received.Timestamp, "empty timestamp")
		bp.received[i].Timestamp = nil
	}

	// We use assert.ElementsMatch because the etags may not be
	// reported in exactly the same order they were fetched.
	assert.Empty(t, cmp.Diff([]*modelpb.APMEvent{
		{
			Labels: modelpb.Labels{"etag": {Value: "abc123"}},
			Metricset: &modelpb.Metricset{
				Name: "agent_config",
				Samples: []*modelpb.MetricsetSample{
					{Name: "agent_config_applied", Value: 1},
				},
			},
		},
		{
			Labels: modelpb.Labels{"etag": {Value: "def456"}},
			Metricset: &modelpb.Metricset{
				Name: "agent_config",
				Samples: []*modelpb.MetricsetSample{
					{Name: "agent_config_applied", Value: 1},
				},
			},
		},
	}, bp.received,
		cmpopts.SortSlices(func(e1 *modelpb.APMEvent, e2 *modelpb.APMEvent) bool {
			return e1.Labels["etag"].Value < e2.Labels["etag"].Value
		}),
		protocmp.Transform()),
	)
}

type fauxFetcher struct{}

func (f fauxFetcher) Fetch(_ context.Context, q Query) (Result, error) {
	if q.Service.Name == "non_matching" {
		return Result{Source: Source{Etag: EtagSentinel}}, nil
	}
	if q.Etag == "old-etag" {
		return Result{
			Source: Source{
				Etag: "new-etag",
			},
		}, nil
	}
	return Result{
		Source: Source{
			Etag: q.Etag,
		},
	}, nil
}

type batchProcessor struct {
	receivedc chan struct{}
	received  []*modelpb.APMEvent
	mu        sync.Mutex
}

func (p *batchProcessor) ProcessBatch(_ context.Context, b *modelpb.Batch) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, event := range *b {
		p.received = append(p.received, event)
	}
	p.receivedc <- struct{}{}
	return nil
}
