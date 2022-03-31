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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestPlaceholderBatchProcessor(t *testing.T) {
	p := new(modelprocessor.Placeholder)
	batch := model.Batch{model.APMEvent{
		Timestamp: time.Now(),
		DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		},
		Metricset: &model.Metricset{DocCount: 1},
	}}
	ctx := context.Background()

	err := p.ProcessBatch(ctx, &batch)
	require.NoError(t, err)
	assert.Equal(t, int64(1), batch[0].Metricset.DocCount)

	p.Set(modelprocessor.UnsetIndexDocCountField{})
	err = p.ProcessBatch(ctx, &batch)
	require.NoError(t, err)
	assert.Equal(t, int64(0), batch[0].Metricset.DocCount)
}
