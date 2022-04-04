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
	"testing"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetDefaultSpanType(t *testing.T) {
	processor := modelprocessor.SetUnknownSpanType{}
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.MetricsetProcessor},
		model.APMEvent{Processor: model.MetricsetProcessor},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.SpanProcessor},
		model.APMEvent{Processor: model.SpanProcessor},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.TransactionProcessor},
		model.APMEvent{Processor: model.TransactionProcessor},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.SpanProcessor, Span: &model.Span{Type: "known"}},
		model.APMEvent{Processor: model.SpanProcessor, Span: &model.Span{Type: "known"}},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.TransactionProcessor, Transaction: &model.Transaction{Type: "known"}},
		model.APMEvent{Processor: model.TransactionProcessor, Transaction: &model.Transaction{Type: "known"}},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.SpanProcessor, Span: &model.Span{Type: ""}},
		model.APMEvent{Processor: model.SpanProcessor, Span: &model.Span{Type: "unknown"}},
	)
	testProcessBatch(t, processor,
		model.APMEvent{Processor: model.TransactionProcessor, Transaction: &model.Transaction{Type: ""}},
		model.APMEvent{Processor: model.TransactionProcessor, Transaction: &model.Transaction{Type: "unknown"}},
	)
}
