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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
)

func TestReportFetch(t *testing.T) {
	interval := time.Millisecond
	receivedc := make(chan struct{})
	defer close(receivedc)
	bp := &batchProcessor{receivedc: receivedc}
	r := NewReporter(fauxFetcher{}, bp, interval)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx)
	query1 := Query{
		Service: Service{Name: "webapp", Environment: "production"},
		Etag:    "abc123",
	}
	applied := true
	query2 := Query{
		Etag:                 "def456",
		MarkAsAppliedByAgent: &applied,
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

	assert.Len(t, bp.received, 2)
	assert.Equal(t, "abc123", bp.received[0].Labels["etag"])
	assert.Equal(t, "def456", bp.received[1].Labels["etag"])
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
	received  []*model.Metricset
}

func (p *batchProcessor) ProcessBatch(_ context.Context, b *model.Batch) error {
	p.received = append(p.received, b.Metricsets...)
	p.receivedc <- struct{}{}
	return nil
}
