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
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetDefaultServiceEnvironment(t *testing.T) {
	nonEmptyServiceEnvironment := model.APMEvent{Service: model.Service{Environment: "nonempty"}}
	defaultServiceEnvironment := model.APMEvent{Service: model.Service{Environment: "default"}}

	processor := modelprocessor.SetDefaultServiceEnvironment{
		DefaultServiceEnvironment: "default",
	}
	testProcessBatch(t, &processor, nonEmptyServiceEnvironment, nonEmptyServiceEnvironment)
	testProcessBatch(t, &processor, model.APMEvent{}, defaultServiceEnvironment)
}

func testProcessBatch(t *testing.T, processor model.BatchProcessor, in, out model.APMEvent) {
	t.Helper()

	batch := &model.Batch{in}
	err := processor.ProcessBatch(context.Background(), batch)
	require.NoError(t, err)

	expected := &model.Batch{out}
	assert.Equal(t, expected, batch)
}
