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

	"github.com/elastic/apm-data/model"
)

// SetUnknownSpanType is a model.BatchProcessor that sets span.type
// or transaction.type to "unknown" for events without one already set.
type SetUnknownSpanType struct{}

// ProcessBatch sets a default span.type or transaction.type for events without one already set.
func (SetUnknownSpanType) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for i := range *b {
		event := &(*b)[i]
		switch event.Processor {
		case model.SpanProcessor:
			if event.Span != nil && event.Span.Type == "" {
				event.Span.Type = "unknown"
			}
		case model.TransactionProcessor:
			if event.Transaction != nil && event.Transaction.Type == "" {
				event.Transaction.Type = "unknown"
			}
		}
	}
	return nil
}
