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

package modelprocessor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestNewDropUnsampled(t *testing.T) {
	for _, dropRUM := range []bool{false, true} {
		registry := monitoring.NewRegistry()
		batchProcessor := modelprocessor.NewDropUnsampled(registry, dropRUM)

		rumAgent := model.Agent{Name: "rum-js"}
		t1 := &model.Transaction{ID: "t1", Sampled: false}
		t2 := &model.Transaction{ID: "t2", Sampled: true}
		t3 := &model.Transaction{ID: "t3", Sampled: false}
		t4 := &model.Transaction{ID: "t4", Sampled: true}
		t5 := &model.Transaction{ID: "t5", Sampled: false}

		batch := model.Batch{{
			Processor:   model.TransactionProcessor,
			Transaction: t1,
		}, {
			Processor:   model.TransactionProcessor,
			Transaction: t2,
		}, {
			Processor: model.ErrorProcessor,
			// Transaction.Sampled should be disregarded, as
			// Processor == ErrorProcessor, i.e. this is an
			// error event with the transaction.sampled field.
			Transaction: &model.Transaction{},
		}, {
			Processor:   model.TransactionProcessor,
			Transaction: t3,
		}, {
			Processor:   model.TransactionProcessor,
			Transaction: t4,
		}, {
			Agent:       rumAgent,
			Processor:   model.TransactionProcessor,
			Transaction: t5,
		}}

		err := batchProcessor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)

		var expectedTransactionsDropped int64 = 3
		expectedRemainingBatch := model.Batch{
			{Processor: model.TransactionProcessor, Transaction: t4},
			{Processor: model.TransactionProcessor, Transaction: t2},
			{Processor: model.ErrorProcessor, Transaction: &model.Transaction{}},
		}
		if !dropRUM {
			expectedTransactionsDropped--
			expectedRemainingBatch = append(expectedRemainingBatch, model.APMEvent{
				Agent: rumAgent, Processor: model.TransactionProcessor, Transaction: t5,
			})
		}

		// Note: this processor is not order-preserving.
		assert.ElementsMatch(t, expectedRemainingBatch, batch)
		expectedMonitoring := monitoring.MakeFlatSnapshot()
		expectedMonitoring.Ints["sampling.transactions_dropped"] = expectedTransactionsDropped
		snapshot := monitoring.CollectFlatSnapshot(registry, monitoring.Full, false /* expvar*/)
		assert.Equal(t, expectedMonitoring, snapshot)
	}
}
