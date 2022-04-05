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

	"github.com/elastic/apm-server/model"
)

// UnsetIndexDocCountField is a BatchProcessor that unsets the DocCount field
// on a model.APMEvent.Metricset.
type UnsetIndexDocCountField struct{}

// ProcessBatch unsets DocCount on model.APMEvent.Metricset.
func (UnsetIndexDocCountField) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for j := range *b {
		if (&(*b)[j]).Metricset != nil {
			(&(*b)[j]).Metricset.DocCount = 0
		}
	}
	return nil
}
