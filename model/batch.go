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

func (b *Batch) Transformables() []transform.Transformable {
	transformables := make([]transform.Transformable, 0, b.Len())
	for i := range b.Transactions {
		transformables = append(transformables, &b.Transactions[i])
	}
	for i := range b.Spans {
		transformables = append(transformables, &b.Spans[i])
	}
	for i := range b.Metricsets {
		transformables = append(transformables, &b.Metricsets[i])
	}
	for i := range b.Errors {
		transformables = append(transformables, &b.Errors[i])
	}
	return transformables
}
