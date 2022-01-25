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
	"sync"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/model"
)

// EventCounter is a model.BatchProcessor that counts the number of events processed,
// recording the counts as metrics in a monitoring.Registry.
//
// Metrics are named after the event type: `processor.<processor.event>.transformations`.
// These metrics are used to populate the "Processed Events" graphs in Stack Monitoring.
type EventCounter struct {
	registry *monitoring.Registry

	mu            sync.RWMutex
	eventCounters map[string]*monitoring.Int
}

// NewEventCounter returns an EventCounter that counts events processed, recording
// them as `<processor.event>.transformations` under the given registry.
func NewEventCounter(registry *monitoring.Registry) *EventCounter {
	return &EventCounter{
		registry:      registry,
		eventCounters: make(map[string]*monitoring.Int),
	}
}

// ProcessBatch counts events in b, grouping by APMEvent.Processor.Event.
func (c *EventCounter) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		pe := event.Processor.Event
		if pe == "" {
			// We want to know if we're processing receiving events
			// without a known type.
			pe = "unknown"
		}
		c.mu.RLock()
		eventCounter := c.eventCounters[pe]
		c.mu.RUnlock()
		if eventCounter == nil {
			c.mu.Lock()
			eventCounter = c.eventCounters[pe]
			if eventCounter == nil {
				// Metric may exist in the registry but not in our map,
				// so first check if it exists before attempting to create.
				name := "processor." + pe + ".transformations"
				var ok bool
				eventCounter, ok = c.registry.Get(name).(*monitoring.Int)
				if !ok {
					eventCounter = monitoring.NewInt(c.registry, name)
				}
				c.eventCounters[pe] = eventCounter
			}
			c.mu.Unlock()
		}
		eventCounter.Inc()
	}
	return nil
}
