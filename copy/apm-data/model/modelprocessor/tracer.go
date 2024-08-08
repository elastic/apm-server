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

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/apm-data/model/modelpb"
)

const instrumentationName = "github.com/elastic/apm-data/model/modelprocessor"

// Tracer is a model.BatchProcessor that wraps another processor within a
// transaction.
type Tracer struct {
	tracer    trace.Tracer
	processor modelpb.BatchProcessor
	spanName  string
}

// NewTracer returns a Tracer that emits transactions for batches processed.
func NewTracer(n string, p modelpb.BatchProcessor, opts ...ConfigOption) *Tracer {
	cfg := newConfig(opts...)

	return &Tracer{
		spanName:  n,
		processor: p,

		tracer: cfg.tracerProvider.Tracer(instrumentationName),
	}
}

// ProcessBatch runs for each batch run, and emits telemetry
func (c *Tracer) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	ctx, span := c.tracer.Start(ctx, c.spanName)
	defer span.End()

	err := c.processor.ProcessBatch(ctx, b)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}
