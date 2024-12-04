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
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-data/model/modelpb"
)

// EventCounter is a model.BatchProcessor that counts the number of events processed,
// recording the counts as metrics in a monitoring.Registry.
//
// Metrics are named after the event type: `processor.<processor.event>.transformations`.
// These metrics are used to populate the "Processed Events" graphs in Stack Monitoring.
type EventCounter struct {
	// Ideally event type would be a dimension, but we can't
	// use dimensions when adapting to libbeat monitoring.
	eventCounters [modelpb.MaxEventType + 1]metric.Int64Counter
}

// NewEventCounter returns an EventCounter that counts events processed, recording
// them as `apm-server.processor.<processor.event>.transformations` under the given registry.
func NewEventCounter(mp metric.MeterProvider) *EventCounter {
	meter := mp.Meter("github.com/elastic/apm-server/internal/model/modelprocessor")
	c := &EventCounter{}
	for i := range c.eventCounters {
		eventType := modelpb.APMEventType(i)
		counter, err := meter.Int64Counter(
			fmt.Sprintf("apm-server.processor.%s.transformations", eventType),
		)
		if err != nil {
			// TODO(axw) return err
			panic(err)
		}
		c.eventCounters[i] = counter
	}
	return c
}

// ProcessBatch counts events in b, grouping by APMEvent.Processor.Event.
func (c *EventCounter) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	for _, event := range *b {
		c.eventCounters[event.Type()].Add(ctx, 1)
	}
	return nil
}
