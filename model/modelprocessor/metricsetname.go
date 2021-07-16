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

	"github.com/elastic/apm-server/model"
)

const (
	spanBreakdownMetricsetName        = "span_breakdown"
	transactionBreakdownMetricsetName = "transaction_breakdown"
	appMetricsetName                  = "app"
)

// SetMetricsetName is a transform.Processor that sets a name for
// metricsets containing well-known agent metrics, such as breakdown
// metrics.
type SetMetricsetName struct{}

// ProcessBatch sets the name for metricsets. Well-defined metrics (breakdowns)
// will be given a specific name, while all other metrics will be given the name
// "app".
func (SetMetricsetName) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		ms := event.Metricset
		if ms == nil || ms.Name != "" || len(ms.Samples) == 0 {
			continue
		}
		ms.Name = appMetricsetName
		if ms.Transaction.Type == "" {
			// Not a breakdown metricset.
			continue
		}
		if _, ok := ms.Samples["span.self_time.count"]; ok {
			ms.Name = spanBreakdownMetricsetName
		} else if _, ok := ms.Samples["transaction.breakdown.count"]; ok {
			ms.Name = transactionBreakdownMetricsetName
		}
	}
	return nil
}
