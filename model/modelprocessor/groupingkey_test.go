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
	"crypto/md5"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetGroupingKey(t *testing.T) {
	tests := map[string]struct {
		input       model.Error
		groupingKey string
	}{
		"empty": {
			input:       model.Error{},
			groupingKey: hashStrings( /*empty*/ ),
		},
		"exception_type_log_parammessage": {
			input: model.Error{
				Exception: &model.Exception{
					Type: "exception_type",
				},
				Log: &model.Log{
					ParamMessage: "log_parammessage",
				},
			},
			groupingKey: hashStrings("exception_type", "log_parammessage"),
		},
		"exception_stacktrace": {
			input: model.Error{
				Exception: &model.Exception{
					Stacktrace: model.Stacktrace{
						{Module: "module", Filename: "filename", Classname: "classname", Function: "func_1"},
						{Filename: "filename", Classname: "classname", Function: "func_2"},
						{ExcludeFromGrouping: true, Function: "func_3"},
					},
					Cause: []model.Exception{{
						Stacktrace: model.Stacktrace{
							{Classname: "classname", Function: "func_4"},
						},
						Cause: []model.Exception{{
							Stacktrace: model.Stacktrace{
								{Function: "func_5"},
							},
						}},
					}, {
						Stacktrace: model.Stacktrace{
							{Function: "func_6"},
						},
					}},
				},
				Log: &model.Log{Stacktrace: model.Stacktrace{{Filename: "abc"}}}, // ignored
			},
			groupingKey: hashStrings(
				"module", "func_1", "filename", "func_2", "classname", "func_4", "func_5", "func_6",
			),
		},
		"log_stacktrace": {
			input: model.Error{
				Log: &model.Log{
					Stacktrace: model.Stacktrace{{Function: "function"}},
				},
			},
			groupingKey: hashStrings("function"),
		},
		"exception_message": {
			input: model.Error{
				Exception: &model.Exception{
					Message: "message_1",
					Cause: []model.Exception{{
						Message: "message_2",
						Cause: []model.Exception{
							{Message: "message_3"},
						},
					}, {
						Message: "message_4",
					}},
				},
				Log: &model.Log{Message: "log_message"}, // ignored
			},
			groupingKey: hashStrings("message_1", "message_2", "message_3", "message_4"),
		},
		"log_message": {
			input: model.Error{
				Log: &model.Log{Message: "log_message"}, // ignored
			},
			groupingKey: hashStrings("log_message"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			batch := model.Batch{{Error: &test.input}}
			processor := modelprocessor.SetGroupingKey{}
			err := processor.ProcessBatch(context.Background(), &batch)
			assert.NoError(t, err)
			assert.Equal(t, test.groupingKey, batch[0].Error.GroupingKey)
		})
	}

}

func hashStrings(s ...string) string {
	md5 := md5.New()
	for _, s := range s {
		md5.Write([]byte(s))
	}
	return hex.EncodeToString(md5.Sum(nil))
}
