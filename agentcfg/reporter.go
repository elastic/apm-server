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
	"time"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
)

type Reporter struct {
	f        Fetcher
	p        model.BatchProcessor
	interval time.Duration
	logger   *logp.Logger
	resultc  chan Result
}

func NewReporter(f Fetcher, batchProcessor model.BatchProcessor, interval time.Duration) Reporter {
	logger := logp.NewLogger("agentcfg")
	return Reporter{
		f:        f,
		p:        batchProcessor,
		interval: interval,
		logger:   logger,
		resultc:  make(chan Result),
	}
}

func (r Reporter) Fetch(ctx context.Context, query Query) (Result, error) {
	result, err := r.f.Fetch(ctx, query)
	if err != nil {
		return Result{}, err
	}
	// Only report configs when the query etag == current config etag, or
	// when the agent indicates it has been applied.
	if query.Etag == result.Source.Etag || query.MarkAsAppliedByAgent {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case r.resultc <- result:
		}
	}

	return result, err
}

func (r Reporter) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	// applied tracks the etags of agent config that has been applied.
	applied := make(map[string]struct{})
	t := time.NewTicker(r.interval)
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
		batch := make(model.Batch, 0, len(applied))
		for etag := range applied {
			batch = append(batch, model.APMEvent{
<<<<<<< HEAD
				Labels: common.MapStr{"etag": etag},
=======
				Timestamp: time.Now(),
				Processor: model.MetricsetProcessor,
				Labels:    common.MapStr{"etag": etag},
>>>>>>> da26f4f5 (fix: add missing timestamp to agent_config metric (#6382))
				Metricset: &model.Metricset{
					Name: "agent_config",
					Samples: map[string]model.MetricsetSample{
						"agent_config_applied": {Value: 1},
					},
				},
			})
		}
		// Reset applied map, so that we report only configs applied
		// during a given iteration.
		applied = make(map[string]struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.p.ProcessBatch(ctx, &batch); err != nil {
				r.logger.Errorf("error sending applied agent configs to kibana: %v", err)
			}
		}()
	}
}
