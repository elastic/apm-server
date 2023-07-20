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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/apmtest"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestTracer(t *testing.T) {
	for _, tt := range []struct {
		name string

		processor modelpb.ProcessBatchFunc

		expectedErr error
	}{
		{
			name: "with a successful parent processor",

			processor: func(context.Context, *modelpb.Batch) error {
				return nil
			},
		},
		{
			name: "with a failing parent processor",

			processor: func(context.Context, *modelpb.Batch) error {
				return errors.New("failure")
			},

			expectedErr: errors.New("failure"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rt := apmtest.NewRecordingTracer()
			tx := rt.Tracer.StartTransaction("testTx", "")
			ctx := apm.ContextWithTransaction(context.Background(), tx)

			processor := NewTracer("testSpan", tt.processor)
			err := processor.ProcessBatch(ctx, &modelpb.Batch{})
			assert.Equal(t, tt.expectedErr, err)

			tx.End()
			rt.Tracer.Flush(nil)
			payloads := rt.Payloads()
			assert.Len(t, payloads.Transactions, 1)
			assert.Len(t, payloads.Spans, 1)

			assert.Equal(t, "testTx", payloads.Transactions[0].Name)
			assert.Equal(t, "testSpan", payloads.Spans[0].Name)

			if tt.expectedErr != nil {
				assert.Equal(t, "failure", payloads.Transactions[0].Outcome)
			}
		})
	}
}
