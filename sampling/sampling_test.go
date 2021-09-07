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

package sampling_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sampling"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestNewDiscardUnsampledBatchProcessor(t *testing.T) {
	batchProcessor := sampling.NewDiscardUnsampledBatchProcessor()

	t1 := &model.Transaction{Sampled: false}
	t2 := &model.Transaction{Sampled: true}
	span := &model.Span{}
	t3 := &model.Transaction{Sampled: false}
	t4 := &model.Transaction{Sampled: true}

	batch := model.Batch{{
		Processor:   model.TransactionProcessor,
		Transaction: t1,
	}, {
		Processor:   model.TransactionProcessor,
		Transaction: t2,
	}, {
		Processor: model.SpanProcessor,
		Span:      span,
		// Transaction.Sampled should be disregarded,
		// as Processor == SpanProcessor, i.e. this is
		// a span event with transaction fields.
		Transaction: &model.Transaction{},
	}, {
		Processor:   model.TransactionProcessor,
		Transaction: t3,
	}, {
		Processor:   model.TransactionProcessor,
		Transaction: t4,
	}}

	err := batchProcessor.ProcessBatch(context.Background(), &batch)
	assert.NoError(t, err)

	// Note: this processor is not order-preserving.
	assert.Equal(t, model.Batch{
		{Processor: model.TransactionProcessor, Transaction: t4},
		{Processor: model.TransactionProcessor, Transaction: t2},
		{Processor: model.SpanProcessor, Span: span, Transaction: &model.Transaction{}},
	}, batch)

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["transactions_dropped"] = 2

	snapshot := monitoring.CollectFlatSnapshot(
		monitoring.GetRegistry("apm-server.sampling"),
		monitoring.Full,
		false, // expvar
	)
	assert.Equal(t, expectedMonitoring, snapshot)
}
