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

package sampling

import (
	"context"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var (
	monitoringRegistry         = monitoring.Default.NewRegistry("apm-server.sampling")
	transactionsDroppedCounter = monitoring.NewInt(monitoringRegistry, "transactions_dropped")
)

// NewDiscardUnsampledReporter returns a publish.Reporter which discards
// unsampled transactions before deferring to reporter.
//
// The returned publish.Reporter does not guarantee order preservation of
// reported events.
func NewDiscardUnsampledReporter(reporter publish.Reporter) publish.Reporter {
	return func(ctx context.Context, req publish.PendingReq) error {
		var dropped int64
		events := req.Transformables
		for i := 0; i < len(events); {
			tx, ok := events[i].(*model.Transaction)
			if !ok || tx.Sampled {
				i++
				continue
			}
			n := len(req.Transformables)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
			dropped++
		}
		if dropped > 0 {
			transactionsDroppedCounter.Add(dropped)
		}
		req.Transformables = events
		return reporter(ctx, req)
	}
}
