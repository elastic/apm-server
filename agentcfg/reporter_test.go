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

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
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
	r.Fetch(ctx, query1)
	r.Fetch(ctx, query2)
	r.Fetch(ctx, query3)
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
		bp.received[i].Timestamp = time.Time{}
	}

	// We use assert.ElementsMatch because the etags may not be
	// reported in exactly the same order they were fetched.
	assert.ElementsMatch(t, []model.APMEvent{
		{
			Processor: model.MetricsetProcessor,
			Labels:    common.MapStr{"etag": "abc123"},
			Metricset: &model.Metricset{
				Name: "agent_config",
				Samples: map[string]model.MetricsetSample{
					"agent_config_applied": {Value: 1},
				},
			},
		},
		{
			Processor: model.MetricsetProcessor,
			Labels:    common.MapStr{"etag": "def456"},
			Metricset: &model.Metricset{
				Name: "agent_config",
				Samples: map[string]model.MetricsetSample{
					"agent_config_applied": {Value: 1},
				},
			},
		},
	}, bp.received)
}

type fauxFetcher struct{}

func (f fauxFetcher) Fetch(_ context.Context, q Query) (Result, error) {
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
	received  []model.APMEvent
	mu        sync.Mutex
}

func (p *batchProcessor) ProcessBatch(_ context.Context, b *model.Batch) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, event := range *b {
		p.received = append(p.received, event)
	}
	p.receivedc <- struct{}{}
	return nil
}
