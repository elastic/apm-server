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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetExcludeFromGrouping(t *testing.T) {
	processor := modelprocessor.SetExcludeFromGrouping{
		Pattern: regexp.MustCompile("foo"),
	}

	tests := []struct {
		input, output model.Batch
	}{{
		input:  model.Batch{{Error: &model.Error{}}, {Transaction: &model.Transaction{}}},
		output: model.Batch{{Error: &model.Error{}}, {Transaction: &model.Transaction{}}},
	}, {
		input: model.Batch{{
			Span: &model.Span{
				Stacktrace: model.Stacktrace{
					{Filename: "foo.go"},
					{Filename: "bar.go"},
					{},
				},
			},
		}},
		output: model.Batch{{
			Span: &model.Span{
				Stacktrace: model.Stacktrace{
					{ExcludeFromGrouping: true, Filename: "foo.go"},
					{Filename: "bar.go"},
					{},
				},
			},
		}},
	}, {
		input: model.Batch{{
			Error: &model.Error{
				Log: &model.ErrorLog{
					Stacktrace: model.Stacktrace{
						{Filename: "foo.go"},
					},
				},
			},
		}, {
			Error: &model.Error{
				Exception: &model.Exception{
					Stacktrace: model.Stacktrace{
						{Filename: "foo.go"},
					},
					Cause: []model.Exception{{
						Stacktrace: model.Stacktrace{
							{Filename: "foo.go"},
						},
					}},
				},
			},
		}},
		output: model.Batch{{
			Error: &model.Error{
				Log: &model.ErrorLog{
					Stacktrace: model.Stacktrace{
						{ExcludeFromGrouping: true, Filename: "foo.go"},
					},
				},
			},
		}, {
			Error: &model.Error{
				Exception: &model.Exception{
					Stacktrace: model.Stacktrace{
						{ExcludeFromGrouping: true, Filename: "foo.go"},
					},
					Cause: []model.Exception{{
						Stacktrace: model.Stacktrace{
							{ExcludeFromGrouping: true, Filename: "foo.go"},
						},
					}},
				},
			},
		}},
	}}

	for _, test := range tests {
		err := processor.ProcessBatch(context.Background(), &test.input)
		assert.NoError(t, err)
		assert.Equal(t, test.output, test.input)
	}

}
