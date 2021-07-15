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

// MetadataProcessorFunc is a function type which implements model.BatchProcessor
// by processing the metadata in each event in the batch.
type MetadataProcessorFunc func(ctx context.Context, meta *model.Metadata) error

// ProcessBatch calls f with the metadata of each event in b.
func (f MetadataProcessorFunc) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		switch {
		case event.Transaction != nil:
			if err := f(ctx, &event.Transaction.Metadata); err != nil {
				return err
			}
		case event.Span != nil:
			if err := f(ctx, &event.Span.Metadata); err != nil {
				return err
			}
		case event.Metricset != nil:
			if err := f(ctx, &event.Metricset.Metadata); err != nil {
				return err
			}
		case event.Error != nil:
			if err := f(ctx, &event.Error.Metadata); err != nil {
				return err
			}
		case event.ProfileSample != nil:
			if err := f(ctx, &event.ProfileSample.Metadata); err != nil {
				return err
			}
		}
	}
	return nil
}
