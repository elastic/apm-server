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

package otel

import (
	"fmt"

	"github.com/elastic/apm-server/model"
)

type transactionBuilder struct {
	*model.Transaction
	httpURL    string
	httpHost   string
	httpScheme string
}

func (tx *transactionBuilder) setFramework(name, version string) {
	if name == "" {
		return
	}
	tx.Metadata.Service.Framework.Name = name
	tx.Metadata.Service.Framework.Version = version
}

func (tx *transactionBuilder) setHTTPMethod(method string) {
	tx.ensureHTTPRequest()
	tx.HTTP.Request.Method = method
}

func (tx *transactionBuilder) setHTTPScheme(scheme string) {
	tx.ensureHTTP()
	tx.httpScheme = scheme
}

func (tx *transactionBuilder) setHTTPURL(httpURL string) {
	tx.ensureHTTP()
	tx.httpURL = httpURL
}

func (tx *transactionBuilder) setHTTPHost(hostport string) {
	tx.ensureHTTP()
	tx.httpHost = hostport
}

func (tx transactionBuilder) setHTTPVersion(version string) {
	tx.ensureHTTP()
	tx.HTTP.Version = version
}

func (tx transactionBuilder) setHTTPRemoteAddr(remoteAddr string) {
	tx.ensureHTTPRequest()
	if tx.HTTP.Request.Socket == nil {
		tx.HTTP.Request.Socket = &model.Socket{}
	}
	tx.HTTP.Request.Socket.RemoteAddress = remoteAddr
}

func (tx *transactionBuilder) setHTTPStatusCode(statusCode int) {
	tx.ensureHTTP()
	if tx.HTTP.Response == nil {
		tx.HTTP.Response = &model.Resp{}
	}
	tx.HTTP.Response.MinimalResp.StatusCode = statusCode
	if tx.Outcome == outcomeUnknown {
		if statusCode >= 500 {
			tx.Outcome = outcomeFailure
		} else {
			tx.Outcome = outcomeSuccess
		}
	}
	if tx.Result == "" {
		tx.Result = httpStatusCodeResult(statusCode)
	}
}

func (tx *transactionBuilder) ensureHTTPRequest() {
	tx.ensureHTTP()
	if tx.HTTP.Request == nil {
		tx.HTTP.Request = &model.Req{}
	}
}

func (tx *transactionBuilder) ensureHTTP() {
	if tx.HTTP == nil {
		tx.HTTP = &model.Http{}
	}
}

var standardStatusCodeResults = [...]string{
	"HTTP 1xx",
	"HTTP 2xx",
	"HTTP 3xx",
	"HTTP 4xx",
	"HTTP 5xx",
}

// httpStatusCodeResult returns the transaction result value to use for the
// given HTTP status code.
func httpStatusCodeResult(statusCode int) string {
	switch i := statusCode / 100; i {
	case 1, 2, 3, 4, 5:
		return standardStatusCodeResults[i-1]
	}
	return fmt.Sprintf("HTTP %d", statusCode)
}
