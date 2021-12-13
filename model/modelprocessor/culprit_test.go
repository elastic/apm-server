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

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetCulprit(t *testing.T) {
	tests := []struct {
		input   model.Error
		culprit string
	}{{
		input:   model.Error{},
		culprit: "",
	}, {
		input:   model.Error{Culprit: "already_set"},
		culprit: "already_set",
	}, {
		input: model.Error{
			Culprit: "already_set",
			Log: &model.ErrorLog{
				Stacktrace: model.Stacktrace{{SourcemapUpdated: false, Filename: "foo.go"}},
			},
		},
		culprit: "already_set",
	}, {
		input: model.Error{
			Culprit: "already_set",
			Log: &model.ErrorLog{
				Stacktrace: model.Stacktrace{{SourcemapUpdated: true, LibraryFrame: true, Filename: "foo.go"}},
			},
		},
		culprit: "already_set",
	}, {
		input: model.Error{
			Culprit: "already_set",
			Log: &model.ErrorLog{
				Stacktrace: model.Stacktrace{
					{SourcemapUpdated: true, LibraryFrame: true, Filename: "foo.go"},
					{SourcemapUpdated: true, LibraryFrame: false, Filename: "foo2.go"},
				},
			},
		},
		culprit: "foo2.go",
	}, {
		input: model.Error{
			Culprit: "already_set",
			Log: &model.ErrorLog{
				Stacktrace: model.Stacktrace{{SourcemapUpdated: true, LibraryFrame: true, Filename: "foo.go"}},
			},
			Exception: &model.Exception{
				Stacktrace: model.Stacktrace{{SourcemapUpdated: true, LibraryFrame: false, Filename: "foo2.go"}},
			},
		},
		culprit: "foo2.go",
	}, {
		input: model.Error{
			Log: &model.ErrorLog{
				Stacktrace: model.Stacktrace{
					{SourcemapUpdated: true, Classname: "AbstractFactoryManagerBean", Function: "toString"},
				},
			},
		},
		culprit: "AbstractFactoryManagerBean in toString",
	}}

	for _, test := range tests {
		batch := model.Batch{{Error: &test.input}}
		processor := modelprocessor.SetCulprit{}
		err := processor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)
		assert.Equal(t, test.culprit, batch[0].Error.Culprit)
	}

}
