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
	"time"

	"github.com/elastic/apm-server/model"
)

type Reporter struct {
	f       Fetcher
	p       model.BatchProcessor
	resultc chan Result

	// keeps track of the agent_configs that have been queried and applied
	// to agents.
	applied map[string]struct{}
}

func NewReporter(f Fetcher, batchProcessor model.BatchProcessor) Reporter {
	return Reporter{
		f:       f,
		p:       batchProcessor,
		applied: make(map[string]struct{}),
		resultc: make(chan Result),
	}
}

func (r Reporter) Fetch(ctx context.Context, query Query) (Result, error) {
	result, err := r.f.Fetch(ctx, query)
	if err == nil {
		r.resultc <- result
	}

	return result, err
}

func (r Reporter) Run(ctx context.Context) error {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-r.resultc:
			if _, ok := r.applied[result.Source.Etag]; !ok {
				r.applied[result.Source.Etag] = struct{}{}
			}
			continue
		case <-t.C:
		}
		batch := new(model.Batch)
		now := time.Now()
		for etag := range r.applied {
			var m *model.Metricset
			m.Labels["etag"] = etag
			m.Name = "direct.agent.config.applied"
			m.Timestamp = now
			batch.Metricsets = append(batch.Metricsets, m)
		}
		if err := r.p.ProcessBatch(ctx, batch); err != nil {
			// TODO: Log error?
		}
	}
}
