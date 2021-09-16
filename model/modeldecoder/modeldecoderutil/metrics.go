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

package modeldecoderutil

import (
	"time"

	"github.com/elastic/apm-server/model"
)

// SetInternalMetrics extracts well-known internal metrics from event.Metricset.Samples,
// setting the appropriate field on event.Transaction and event.Span (if non-nil) and
// finally setting event.Metricset.Samples to nil.
//
// Any unknown metrics sent by agents in a metricset with transaction.* set will be
// silently discarded.
func SetInternalMetrics(event *model.APMEvent) {
	if event.Transaction == nil {
		// Not an internal metricset.
		return
	}
	for k, v := range event.Metricset.Samples {
		switch k {
		case "transaction.breakdown.count":
			event.Transaction.BreakdownCount = int(v.Value)
		case "span.self_time.count":
			if event.Span != nil {
				event.Span.SelfTime.Count = int(v.Value)
			}
		case "span.self_time.sum.us":
			if event.Span != nil {
				event.Span.SelfTime.Sum = time.Duration(v.Value * 1000)
			}
		}
	}
	event.Metricset.Samples = nil
}
