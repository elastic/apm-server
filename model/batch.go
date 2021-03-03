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

package model

import (
	"context"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
)

// BatchProcessor can be used to process a batch of events, giving the
// opportunity to update, add or remove events.
type BatchProcessor interface {
	// ProcessBatch is called with a batch of events for processing.
	//
	// Processing may involve anything, e.g. modifying, adding, removing,
	// aggregating, or publishing events.
	ProcessBatch(context.Context, *Batch) error
}

// ProcessBatchFunc is a function type that implements BatchProcessor.
type ProcessBatchFunc func(context.Context, *Batch) error

// ProcessBatch calls f(ctx, b)
func (f ProcessBatchFunc) ProcessBatch(ctx context.Context, b *Batch) error {
	return f(ctx, b)
}

type Batch struct {
	Transactions []*Transaction
	Spans        []*Span
	Metricsets   []*Metricset
	Errors       []*Error
	Profiles     []*PprofProfile
}

// Reset resets the batch to be empty, but it retains the underlying storage.
func (b *Batch) Reset() {
	b.Transactions = b.Transactions[:0]
	b.Spans = b.Spans[:0]
	b.Metricsets = b.Metricsets[:0]
	b.Errors = b.Errors[:0]
	b.Profiles = b.Profiles[:0]
}

func (b *Batch) Len() int {
	if b == nil {
		return 0
	}
	return len(b.Transactions) + len(b.Spans) + len(b.Metricsets) + len(b.Errors) + len(b.Profiles)
}

func (b *Batch) Transform(ctx context.Context, cfg *transform.Config) []beat.Event {
	events := make([]beat.Event, 0, b.Len())
	for _, event := range b.Transactions {
		events = append(events, event.Transform(ctx, cfg)...)
	}
	for _, event := range b.Spans {
		events = append(events, event.Transform(ctx, cfg)...)
	}
	for _, event := range b.Metricsets {
		events = append(events, event.Transform(ctx, cfg)...)
	}
	for _, event := range b.Errors {
		events = append(events, event.Transform(ctx, cfg)...)
	}
	for _, event := range b.Profiles {
		events = append(events, event.Transform(ctx, cfg)...)
	}
	return events
}
