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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestNewDropUnsampled(t *testing.T) {
	for _, dropRUM := range []bool{false, true} {
		var dropped int64
		batchProcessor := modelprocessor.NewDropUnsampled(dropRUM, func(i int64) {
			dropped += i
		})

		rumAgent := modelpb.Agent{Name: "rum-js"}
		t1 := &modelpb.Transaction{Type: "type", Id: "t1", Sampled: false}
		t2 := &modelpb.Transaction{Type: "type", Id: "t2", Sampled: true}
		t3 := &modelpb.Transaction{Type: "type", Id: "t3", Sampled: false}
		t4 := &modelpb.Transaction{Type: "type", Id: "t4", Sampled: true}
		t5 := &modelpb.Transaction{Type: "type", Id: "t5", Sampled: false}

		batch := modelpb.Batch{{
			Transaction: t1,
		}, {
			Transaction: t2,
		}, {
			// Transaction.Sampled should be disregarded, as this is
			// an error event with the transaction.sampled field.
			Error:       &modelpb.Error{},
			Transaction: &modelpb.Transaction{},
		}, {
			Transaction: t3,
		}, {
			Transaction: t4,
		}, {
			Agent:       &rumAgent,
			Transaction: t5,
		}}

		err := batchProcessor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)

		var expectedTransactionsDropped int64 = 3
		expectedRemainingBatch := modelpb.Batch{
			{Transaction: t4},
			{Transaction: t2},
			{Error: &modelpb.Error{}, Transaction: &modelpb.Transaction{}},
		}
		if !dropRUM {
			expectedTransactionsDropped--
			expectedRemainingBatch = append(expectedRemainingBatch, &modelpb.APMEvent{
				Agent: &rumAgent, Transaction: t5,
			})
		}

		// Note: this processor is not order-preserving.
		assert.ElementsMatch(t, expectedRemainingBatch, batch)
		assert.Equal(t, expectedTransactionsDropped, dropped)
	}
}
