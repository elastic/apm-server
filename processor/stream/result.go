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
	"fmt"
	"strings"
)

type Error struct {
	Type     StreamError `json:"-"`
	Message  string      `json:"message"`
	Document string      `json:"document"`
}

func (s *Error) Error() string {
	if s.Document != "" {
		return fmt.Sprintf("%s [%s]", s.Message, string(s.Document))
	}
	return s.Message
}

type StreamError int

const (
	QueueFullErrType StreamError = iota
	ProcessingTimeoutErrType
	InvalidInputErrType
	ShuttingDownErrType
	ServerErrType
)

const (
	errorsLimit = 5
)

type Result struct {
	Accepted int      `json:"accepted"`
	Errors   []*Error `json:"errors,omitempty"`
}

func (r *Result) LimitedAdd(err error) {
	if len(r.Errors) < errorsLimit {
		if e, ok := err.(*Error); ok {
			r.Errors = append(r.Errors, e)
		} else {
			r.Errors = append(r.Errors, &Error{Message: err.Error(), Type: ServerErrType})
		}
	}
}

func (r *Result) String() string {
	errorList := []string{}
	for _, e := range r.Errors {
		errorList = append(errorList, e.Error())
	}
	return strings.Join(errorList, ", ")
}
