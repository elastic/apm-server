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
	Transactions []*Transaction
	Spans        []*Span
	Metricsets   []*Metricset
	Errors       []*Error
}

// Reset resets the batch to be empty, but it retains the underlying storage.
func (b *Batch) Reset() {
	b.Transactions = b.Transactions[:0]
	b.Spans = b.Spans[:0]
	b.Metricsets = b.Metricsets[:0]
	b.Errors = b.Errors[:0]
}

func (b *Batch) Len() int {
	if b == nil {
		return 0
	}
	return len(b.Transactions) + len(b.Spans) + len(b.Metricsets) + len(b.Errors)
}

func (b *Batch) Transformables() []transform.Transformable {
	transformables := make([]transform.Transformable, 0, b.Len())
	for _, tx := range b.Transactions {
		transformables = append(transformables, tx)
	}
	for _, span := range b.Spans {
		transformables = append(transformables, span)
	}
	for _, metricset := range b.Metricsets {
		transformables = append(transformables, metricset)
	}
	for _, err := range b.Errors {
		transformables = append(transformables, err)
	}
	return transformables
}
