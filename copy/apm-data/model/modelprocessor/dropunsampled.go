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

	"github.com/elastic/apm-data/model/modelpb"
)

// NewDropUnsampled returns a modelpb.BatchProcessor which drops unsampled transaction events,
// and runs the specified callback.
//
// If dropRUM is false, only non-RUM unsampled transaction events are dropped; otherwise all
// unsampled transaction events are dropped.
//
// This modelpb.BatchProcessor does not guarantee order preservation of the remaining events.
func NewDropUnsampled(dropRUM bool, droppedCallback func(int64)) modelpb.BatchProcessor {
	return modelpb.ProcessBatchFunc(func(_ context.Context, batch *modelpb.Batch) error {
		events := *batch
		for i := 0; i < len(events); {
			event := events[i]
			if !shouldDropUnsampled(event, dropRUM) {
				i++
				continue
			}
			n := len(events)
			events[i], events[n-1] = events[n-1], events[i]
			events = events[:n-1]
		}
		if dropped := len(*batch) - len(events); dropped > 0 {
			droppedCallback(int64(dropped))
		}
		*batch = events
		return nil
	})
}

func shouldDropUnsampled(event *modelpb.APMEvent, dropRUM bool) bool {
	if event.Type() != modelpb.TransactionEventType || event.GetTransaction().GetSampled() {
		return false
	}
	return dropRUM || !isRUMAgentName(event.GetAgent().GetName())
}
