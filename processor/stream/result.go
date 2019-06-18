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
	"net/http"
	"strings"

	"github.com/elastic/apm-server/server"

	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	errorsLimit = 5
)

var (
	eventMetrics = monitoring.Default.NewRegistry("apm-server.processor.stream")
	accepted     = monitoring.NewInt(eventMetrics, "accepted")
)

type result struct {
	accepted int
	errors   []error
	code     int
}

func newErroredResult(e error) result {
	return result{errors: []error{e}, code: http.StatusBadRequest}
}

func (r result) Code() int {
	if r.code == 0 {
		return http.StatusAccepted
	}
	return r.code
}

func (r result) Body() interface{} {
	body := map[string]interface{}{
		"accepted": r.accepted,
	}
	if r.IsError() {
		body["errors"] = r.Error()
	}
	return body
}

func (r result) IsError() bool {
	return len(r.errors) > 0
}

func (r *result) add(e *server.Error) {
	if e.Code() > r.code {
		r.code = e.Code()
	}
	r.errors = append(r.errors, e.Err)
}

func (r *result) addAccepted(ct int) {
	r.accepted += ct
	accepted.Add(int64(ct))
}

func (r result) Error() string {
	var errorList []string
	for idx, e := range r.errors {
		if idx == errorsLimit {
			break
		}
		errorList = append(errorList, e.Error())
	}
	return strings.Join(errorList, ", ")
}
