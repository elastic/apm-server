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

package modeldecoder

import (
	"regexp"

	"github.com/pkg/errors"
)

// DecoderError represents an error due to JSON decoding.
type DecoderError struct {
	err error
}

func (e DecoderError) Error() string {
	return errors.Wrap(e.err, "decode error").Error()
}

func (e *DecoderError) Unwrap() error {
	return e.err
}

// ValidationError represents an error due to JSON validation.
type ValidationError struct {
	err error
}

func (e ValidationError) Error() string {
	return errors.Wrap(e.err, "validation error").Error()
}

func (e *ValidationError) Unwrap() error {
	return e.err
}

var jsoniterErrRegexp = regexp.MustCompile(` but found .*error found in .* bigger context.*`)

// NewDecoderErrFromJSONIter returns a DecoderError where
// any text from the original input is stripped,
// when decoded via jsoniter.
func NewDecoderErrFromJSONIter(err error) DecoderError {
	if jsoniterErrRegexp.MatchString(err.Error()) {
		err = errors.New(jsoniterErrRegexp.ReplaceAllString(err.Error(), ""))
	}
	return DecoderError{err}
}

// NewValidationErr returns a ValidationError
func NewValidationErr(err error) ValidationError {
	return ValidationError{err}
}
