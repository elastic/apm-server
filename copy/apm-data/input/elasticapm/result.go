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
	"errors"
)

type Result struct {
	errorsSpace [5]error
	// Errors holds a limited number of errors that occurred while
	// processing the event stream. If the limit is reached, the
	// counters above are still incremented.
	Errors []error
	// Accepted holds the number of valid events accepted.
	Accepted int
	// TooLarge holds the number of events that were rejected due
	// to exceeding the event size limit.
	TooLarge int
	// Invalid holds the number of events that were rejected due
	// to being invalid, excluding those that are counted by TooLarge.
	Invalid int
}

func (r *Result) addError(err error) {
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		if invalid.TooLarge {
			r.TooLarge++
		} else {
			r.Invalid++
		}
	}
	if len(r.Errors) < len(r.errorsSpace) {
		if r.Errors == nil {
			r.Errors = r.errorsSpace[:0]
		}
		r.Errors = append(r.Errors, err)
	}
}

type InvalidInputError struct {
	Message  string
	Document string
	TooLarge bool
}

func (e *InvalidInputError) Error() string {
	return e.Message
}
