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

	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

// Tracer is a model.BatchProcessor that wraps another processor within a
// transaction.
type Tracer struct {
	spanName  string
	processor modelpb.BatchProcessor
}

// NewTracer returns a Tracer that emits transactions for batches processed.
func NewTracer(n string, p modelpb.BatchProcessor) *Tracer {
	return &Tracer{
		spanName:  n,
		processor: p,
	}
}

// ProcessBatch runs for each batch run, and emits telemetry
func (c *Tracer) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	span, ctx := apm.StartSpan(ctx, c.spanName, "")
	defer span.End()

	err := c.processor.ProcessBatch(ctx, b)
	if err != nil {
		if e := apm.CaptureError(ctx, err); e != nil {
			e.Send()
		}
	}

	return err
}
