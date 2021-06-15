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
	"errors"

	"github.com/elastic/beats/v7/libbeat/monitoring"
)

const (
	errorsLimit = 5
)

var (
	m         = monitoring.Default.NewRegistry("apm-server.processor.stream")
	mAccepted = monitoring.NewInt(m, "accepted")
	mInvalid  = monitoring.NewInt(m, "errors.invalid")
	mTooLarge = monitoring.NewInt(m, "errors.toolarge")
)

type Result struct {
	Accepted int
	Errors   []error
}

func (r *Result) LimitedAdd(err error) {
	r.add(err, len(r.Errors) < errorsLimit)
}

func (r *Result) Add(err error) {
	r.add(err, true)
}

func (r *Result) AddAccepted(ct int) {
	r.Accepted += ct
	mAccepted.Add(int64(ct))
}

func (r *Result) add(err error, add bool) {
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		if invalid.TooLarge {
			mTooLarge.Inc()
		} else {
			mInvalid.Inc()
		}
	}
	if add {
		r.Errors = append(r.Errors, err)
	}
}

type InvalidInputError struct {
	TooLarge bool
	Message  string
	Document string
}

func (e *InvalidInputError) Error() string {
	return e.Message
}
