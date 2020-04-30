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
	"github.com/elastic/apm-server/transform"
)

type Batch struct {
	Transactions []Transaction
	Spans        []Span
	Metricsets   []Metricset
	Errors       []Error
}

func (b *Batch) Len() int {
	if b == nil {
		return 0
	}
	return len(b.Transactions) + len(b.Spans) + len(b.Metricsets) + len(b.Errors)
}

func (b *Batch) Expand(b2 *Batch) {
	if b2 == nil {
		return
	}
	b.Transactions = append(b.Transactions, b2.Transactions...)
	b.Spans = append(b.Spans, b2.Spans...)
	b.Metricsets = append(b.Metricsets, b2.Metricsets...)
	b.Errors = append(b.Errors, b2.Errors...)
}

func (b *Batch) Transformables() []transform.Transformable {
	transformables := make([]transform.Transformable, b.Len())
	for i := range b.Transactions {
		transformables[i] = &b.Transactions[i]
	}
	var offset = len(b.Transactions)

	for i := range b.Spans {
		transformables[i+offset] = &b.Spans[i]
	}
	offset += len(b.Spans)

	for i := range b.Metricsets {
		transformables[i+offset] = &b.Metricsets[i]
	}
	offset += len(b.Metricsets)

	for i := range b.Errors {
		transformables[i+offset] = &b.Errors[i]
	}
	return transformables
}
