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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetDefaultServiceEnvironment(t *testing.T) {
	nonEmptyMetadata := model.Metadata{Service: model.Service{Environment: "nonempty"}}
	defaultMetadata := model.Metadata{Service: model.Service{Environment: "default"}}

	processor := modelprocessor.SetDefaultServiceEnvironment{
		DefaultServiceEnvironment: "default",
	}
	testProcessBatchMetadata(t, &processor, nonEmptyMetadata, nonEmptyMetadata)
	testProcessBatchMetadata(t, &processor, model.Metadata{}, defaultMetadata)
}

func testProcessBatchMetadata(t *testing.T, processor model.BatchProcessor, in, out model.Metadata) {
	t.Helper()

	// Check that the model.Batch fields have not changed since this
	// test was last updated, to ensure we process all model types.
	var apmEventFields []string
	typ := reflect.TypeOf(model.APMEvent{})
	for i := 0; i < typ.NumField(); i++ {
		apmEventFields = append(apmEventFields, typ.Field(i).Name)
	}
	assert.ElementsMatch(t, []string{
		"Transaction",
		"Span",
		"Metricset",
		"Error",
		"Profile",
	}, apmEventFields)

	batch := &model.Batch{
		{Transaction: &model.Transaction{Metadata: in}},
		{Span: &model.Span{Metadata: in}},
		{Metricset: &model.Metricset{Metadata: in}},
		{Error: &model.Error{Metadata: in}},
		{Profile: &model.PprofProfile{Metadata: in}},
	}
	err := processor.ProcessBatch(context.Background(), batch)
	require.NoError(t, err)

	expected := &model.Batch{
		{Transaction: &model.Transaction{Metadata: out}},
		{Span: &model.Span{Metadata: out}},
		{Metricset: &model.Metricset{Metadata: out}},
		{Error: &model.Error{Metadata: out}},
		{Profile: &model.PprofProfile{Metadata: out}},
	}
	assert.Equal(t, expected, batch)
}
