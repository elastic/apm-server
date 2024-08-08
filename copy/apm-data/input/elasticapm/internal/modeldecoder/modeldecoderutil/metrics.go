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
	"github.com/elastic/apm-data/model/modelpb"
)

// SetInternalMetrics extracts well-known internal metrics from event.Metricset.Samples,
// setting the appropriate field on event.Span (if non-nil) and finally setting
// event.Metricset.Samples to nil.
//
// Any unknown metrics sent by agents in a metricset with transaction.* set will be
// silently discarded.
//
// SetInternalMetrics returns true if any known metric samples were found, and false
// otherwise. If no known metric samples were found, the caller may opt to omit the
// metricset altogether.
func SetInternalMetrics(event *modelpb.APMEvent) bool {
	if event.Transaction == nil {
		// Not an internal metricset.
		return false
	}
	var haveMetrics bool
	if event.Span != nil {
		for _, v := range event.Metricset.Samples {
			switch v.Name {
			case "span.self_time.count":
				if event.Span.SelfTime == nil {
					event.Span.SelfTime = &modelpb.AggregatedDuration{}
				}
				event.Span.SelfTime.Count = uint64(v.Value)
				haveMetrics = true
			case "span.self_time.sum.us":
				if event.Span.SelfTime == nil {
					event.Span.SelfTime = &modelpb.AggregatedDuration{}
				}
				event.Span.SelfTime.Sum = uint64(v.Value * 1000)
				haveMetrics = true
			}
		}
	}
	if haveMetrics {
		event.Metricset.Name = "span_breakdown"
	}
	event.Metricset.Samples = nil
	return haveMetrics
}
