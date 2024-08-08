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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestSetDefaultServiceEnvironment(t *testing.T) {
	nonEmptyServiceEnvironment := modelpb.APMEvent{Service: &modelpb.Service{Environment: "nonempty"}}
	defaultServiceEnvironment := modelpb.APMEvent{Service: &modelpb.Service{Environment: "default"}}

	processor := modelprocessor.SetDefaultServiceEnvironment{
		DefaultServiceEnvironment: "default",
	}
	testProcessBatch(t, &processor, &nonEmptyServiceEnvironment, &nonEmptyServiceEnvironment)
	testProcessBatch(t, &processor, &modelpb.APMEvent{}, &defaultServiceEnvironment)
}

func testProcessBatch(t *testing.T, processor modelpb.BatchProcessor, in, out *modelpb.APMEvent) {
	t.Helper()

	batch := &modelpb.Batch{in}
	err := processor.ProcessBatch(context.Background(), batch)
	require.NoError(t, err)

	expected := &modelpb.Batch{out}
	assert.Equal(t, expected, batch)
}
