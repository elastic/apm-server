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

package modelprocessor

import (
	"context"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/model"
)

var (
	monitoringRegistry         = monitoring.Default.NewRegistry("apm-server.sampling")
	transactionsDroppedCounter = monitoring.NewInt(monitoringRegistry, "transactions_dropped")
)

// NewDropUnsampled returns a model.BatchProcessor which drops unsampled transaction events,
// and counts them in a metric named `apm-server.sampling.transactions_dropped`.
//
// If dropRUM is false, only non-RUM unsampled transaction events are dropped; otherwise all
// unsampled transaction events are dropped.
//
// This model.BatchProcessor does not guarantee order preservation of the remaining events.
func NewDropUnsampled(dropRUM bool) model.BatchProcessor {
	return model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		events := *batch
		for i := 0; i < len(events); {
			event := events[i]
			if !shouldDropUnsampled(&event, dropRUM) {
				i++
				continue
			}
			n := len(events)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
		}
		if dropped := len(*batch) - len(events); dropped > 0 {
			transactionsDroppedCounter.Add(int64(dropped))
		}
		*batch = events
		return nil
	})
}

func shouldDropUnsampled(event *model.APMEvent, dropRUM bool) bool {
	if event.Processor != model.TransactionProcessor || event.Transaction == nil || event.Transaction.Sampled {
		return false
	}
	return dropRUM || !isRUMAgentName(event.Agent.Name)
}
