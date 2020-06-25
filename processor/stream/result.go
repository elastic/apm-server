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
	"strings"

	"github.com/elastic/beats/v7/libbeat/monitoring"
)

type Error struct {
	Type     StreamError `json:"-"`
	Message  string      `json:"message"`
	Document string      `json:"document,omitempty"`
}

func (s *Error) Error() string {
	return s.Message
}

type StreamError int

const (
	QueueFullErrType StreamError = iota
	InvalidInputErrType
	InputTooLargeErrType
	ShuttingDownErrType
	ServerErrType
	MethodForbiddenErrType
	RateLimitErrType
)

const (
	errorsLimit = 5
)

var (
	m             = monitoring.Default.NewRegistry("apm-server.processor.stream")
	mAccepted     = monitoring.NewInt(m, "accepted")
	monitoringMap = map[StreamError]*monitoring.Int{
		QueueFullErrType:     monitoring.NewInt(m, "errors.queue"),
		InvalidInputErrType:  monitoring.NewInt(m, "errors.invalid"),
		InputTooLargeErrType: monitoring.NewInt(m, "errors.toolarge"),
		ShuttingDownErrType:  monitoring.NewInt(m, "errors.server"),
		ServerErrType:        monitoring.NewInt(m, "errors.closed"),
	}
)

type Result struct {
	Accepted int      `json:"accepted"`
	Errors   []*Error `json:"errors,omitempty"`
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

func (r *Result) Error() string {
	var errorList []string
	for _, e := range r.Errors {
		errorList = append(errorList, e.Error())
	}
	return strings.Join(errorList, ", ")
}

func (r *Result) add(err error, add bool) {
	e, ok := err.(*Error)
	if !ok {
		e = &Error{Message: err.Error(), Type: ServerErrType}
	}
	if add {
		r.Errors = append(r.Errors, e)
	}
	countErr(e.Type)
}

func countErr(e StreamError) {
	if i, ok := monitoringMap[e]; ok {
		i.Inc()
	} else {
		monitoringMap[ServerErrType].Inc()
	}
}
