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
	"github.com/elastic/beats/v7/libbeat/logp"
)

type Reporter struct {
	f       Fetcher
	p       model.BatchProcessor
	logger  *logp.Logger
	resultc chan Result
}

func NewReporter(f Fetcher, batchProcessor model.BatchProcessor) Reporter {
	logger := logp.NewLogger("agentcfg")
	return Reporter{
		f:       f,
		p:       batchProcessor,
		logger:  logger,
		resultc: make(chan Result),
	}
}

func (r Reporter) Fetch(ctx context.Context, query Query) (Result, error) {
	result, err := r.f.Fetch(ctx, query)
	// Only report configs when the query etag == current config etag, or
	// when the agent indicates it has been applied.
	if err == nil && (query.Etag == result.Source.Etag || *query.MarkAsAppliedByAgent) {
		r.resultc <- result
	}

	return result, err
}

func (r Reporter) Run(ctx context.Context) error {
	// keeps track of the agent_configs that have been queried and applied
	// to agents.
	applied := make(map[string]struct{})
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-r.resultc:
			if _, ok := applied[result.Source.Etag]; !ok {
				applied[result.Source.Etag] = struct{}{}
			}
			continue
		case <-t.C:
		}
		batch := new(model.Batch)
		for etag := range applied {
			var m *model.Metricset
			m.Labels["etag"] = etag
			m.Name = "agent_config"
			m.Samples = append(m.Samples, model.Sample{Name: "agent_config_applied", Value: 1})
			batch.Metricsets = append(batch.Metricsets, m)
		}
		// Reset applied map, so that we report only configs applied
		// during a given iteration.
		applied = make(map[string]struct{})
		go func() {
			if err := r.p.ProcessBatch(ctx, batch); err != nil {
				r.logger.Errorf("error sending applied agent configs to kibana: %v", err)
			}
		}()
	}
}
