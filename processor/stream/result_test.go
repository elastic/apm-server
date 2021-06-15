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

package stream

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestResultAdd(t *testing.T) {
	err1 := &InvalidInputError{Message: "err1", Document: "buf1"}
	err2 := &InvalidInputError{Message: "err2", Document: "buf2"}
	err3 := &InvalidInputError{Message: "err3", Document: "buf3"}
	err4 := &InvalidInputError{Message: "err4"}
	err5 := &InvalidInputError{Message: "err5"}
	err6 := &InvalidInputError{Message: "err6"}
	err7 := &InvalidInputError{Message: "err7"}

	result := Result{}
	result.LimitedAdd(err1)
	result.LimitedAdd(err2)
	result.LimitedAdd(err3)
	result.LimitedAdd(err4)
	result.LimitedAdd(err5)
	result.LimitedAdd(err5)
	result.LimitedAdd(err6) // limited, not added
	result.Add(err7)        // unconditionally added

	assert.Len(t, result.Errors, 6)
	assert.Equal(t, []error{err1, err2, err3, err4, err5, err7}, result.Errors)
}

func TestMonitoring(t *testing.T) {
	initialAccepted := mAccepted.Get()
	initialInvalid := mInvalid.Get()
	initialTooLarge := mTooLarge.Get()

	var result Result
	result.AddAccepted(9)
	result.AddAccepted(3)
	for i := 0; i < 10; i++ {
		result.LimitedAdd(&InvalidInputError{TooLarge: false})
	}
	result.LimitedAdd(&InvalidInputError{TooLarge: true})
	result.Add(&InvalidInputError{TooLarge: true})
	result.Add(errors.New("error"))

	assert.Equal(t, int64(12), mAccepted.Get()-initialAccepted)
	assert.Equal(t, int64(10), mInvalid.Get()-initialInvalid)
	assert.Equal(t, int64(2), mTooLarge.Get()-initialTooLarge)
}
