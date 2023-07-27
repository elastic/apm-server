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

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/model/modelprocessor"
)

func TestEventCounter(t *testing.T) {
	batch := modelpb.Batch{
		{},
		{Transaction: &modelpb.Transaction{Type: "transaction_type"}},
		{Span: &modelpb.Span{Type: "span_type"}},
		{Transaction: &modelpb.Transaction{Type: "transaction_type"}},
	}

	expected := monitoring.MakeFlatSnapshot()
	expected.Ints["processor.span.transformations"] = 1
	expected.Ints["processor.transaction.transformations"] = 2

	registry := monitoring.NewRegistry()
	processor := modelprocessor.NewEventCounter(registry)
	err := processor.ProcessBatch(context.Background(), &batch)
	assert.NoError(t, err)
	snapshot := monitoring.CollectFlatSnapshot(registry, monitoring.Full, false)
	assert.Equal(t, expected, snapshot)

}
