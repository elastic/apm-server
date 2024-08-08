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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestSetErrorMessage(t *testing.T) {
	tests := []struct {
		input   *modelpb.Error
		message string
	}{{
		input:   &modelpb.Error{},
		message: "",
	}, {
		input:   &modelpb.Error{Log: &modelpb.ErrorLog{Message: "log_message"}},
		message: "log_message",
	}, {
		input:   &modelpb.Error{Exception: &modelpb.Exception{Message: "exception_message"}},
		message: "exception_message",
	}, {
		input: &modelpb.Error{
			Log:       &modelpb.ErrorLog{},
			Exception: &modelpb.Exception{Message: "exception_message"},
		},
		message: "exception_message",
	}, {
		input: &modelpb.Error{
			Log:       &modelpb.ErrorLog{Message: "log_message"},
			Exception: &modelpb.Exception{Message: "exception_message"},
		},
		message: "log_message",
	}}

	for _, test := range tests {
		batch := modelpb.Batch{{Error: test.input}}
		processor := modelprocessor.SetErrorMessage{}
		err := processor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)
		assert.Equal(t, test.message, batch[0].Message)
	}

}
