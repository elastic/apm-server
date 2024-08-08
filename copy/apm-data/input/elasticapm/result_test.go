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

package elasticapm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultErrors(t *testing.T) {
	err1 := &InvalidInputError{Message: "err1", Document: "buf1"}
	err2 := &InvalidInputError{Message: "err2", Document: "buf2"}
	err3 := &InvalidInputError{Message: "err3", Document: "buf3"}
	err4 := &InvalidInputError{Message: "err4"}
	err5 := &InvalidInputError{Message: "err5"}
	err6 := &InvalidInputError{Message: "err6"}
	err7 := &InvalidInputError{Message: "err7", TooLarge: true}
	err8 := &InvalidInputError{Message: "err8", TooLarge: true}

	result := Result{}
	result.addError(err1)
	result.addError(err2)
	result.addError(err3)
	result.addError(err4)
	result.addError(err5)

	// the following errors are not added due to the limit being
	// reached, but they are still accumulated in counters
	result.addError(err6)
	result.addError(err7)
	result.addError(err8)

	assert.Equal(t, []error{err1, err2, err3, err4, err5}, result.Errors)
	assert.Equal(t, 6, result.Invalid)
	assert.Equal(t, 2, result.TooLarge)
}
