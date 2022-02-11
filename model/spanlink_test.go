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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestSpanLinkFields(t *testing.T) {
	tests := []struct {
		Input    SpanLink
		Expected common.MapStr
	}{{
		Input:    SpanLink{},
		Expected: nil,
	}, {
		Input: SpanLink{
			Span: Span{ID: "span_id"},
		},
		Expected: common.MapStr{
			"span": common.MapStr{"id": "span_id"},
		},
	}, {
		Input: SpanLink{
			Trace: Trace{ID: "trace_id"},
		},
		Expected: common.MapStr{
			"trace": common.MapStr{"id": "trace_id"},
		},
	}, {
		Input: SpanLink{
			Span:  Span{ID: "span_id"},
			Trace: Trace{ID: "trace_id"},
		},
		Expected: common.MapStr{
			"span":  common.MapStr{"id": "span_id"},
			"trace": common.MapStr{"id": "trace_id"},
		},
	}}
	for _, test := range tests {
		output := test.Input.fields()
		assert.Equal(t, test.Expected, output)
	}
}
