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

package request

import (
	"net/http"

	"github.com/pkg/errors"
)

const (
	IdUnset ResultID = "unset"

	IdRequestCount        ResultID = "request.count"
	IdResponseCount       ResultID = "response.count"
	IdResponseErrorsCount ResultID = "response.errors.count"
	IdResponseValidCount  ResultID = "response.valid.count"

	IdResponseValidOK          ResultID = "response.valid.ok"
	IdResponseValidAccepted    ResultID = "response.valid.accepted"
	IdResponseValidNotModified ResultID = "response.valid.notmodified"

	IdResponseErrorsForbidden          ResultID = "response.errors.forbidden"
	IdResponseErrorsUnauthorized       ResultID = "response.errors.unauthorized"
	IdResponseErrorsNotFound           ResultID = "response.errors.notfound"
	IdResponseErrorsInvalidQuery       ResultID = "response.errors.invalidquery"
	IdResponseErrorsRequestTooLarge    ResultID = "response.errors.toolarge"
	IdResponseErrorsDecode             ResultID = "response.errors.decode"
	IdResponseErrorsValidate           ResultID = "response.errors.validate"
	IdResponseErrorsRateLimit          ResultID = "response.errors.ratelimit"
	IdResponseErrorsMethodNotAllowed   ResultID = "response.errors.method"
	IdResponseErrorsFullQueue          ResultID = "response.errors.queue"
	IdResponseErrorsShuttingDown       ResultID = "response.errors.closed"
	IdResponseErrorsServiceUnavailable ResultID = "response.errors.unavailable"
	IdResponseErrorsInternal           ResultID = "response.errors.internal"
)

var (
	OKResponse = Result{
		StatusCode: http.StatusOK,
		Id:         IdResponseValidOK,
	}
	AcceptedResponse = Result{
		StatusCode: http.StatusAccepted,
		Id:         IdResponseValidAccepted,
	}
	InternalErrorResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "internal error"),
			StatusCode: http.StatusInternalServerError,
			Id:         IdResponseErrorsInternal,
		}
	}
	ForbiddenResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "forbidden request"),
			StatusCode: http.StatusForbidden,
			Id:         IdResponseErrorsForbidden,
		}
	}
	UnauthorizedResponse = Result{
		Err:        errors.New("invalid token"),
		StatusCode: http.StatusUnauthorized,
		Id:         IdResponseErrorsUnauthorized,
	}
	RequestTooLargeResponse = Result{
		Err:        errors.New("request body too large"),
		StatusCode: http.StatusRequestEntityTooLarge,
		Id:         IdResponseErrorsRequestTooLarge,
	}
	CannotDecodeResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "data decoding error"),
			StatusCode: http.StatusBadRequest,
			Id:         IdResponseErrorsDecode,
		}
	}
	CannotValidateResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "data validation error"),
			StatusCode: http.StatusBadRequest,
			Id:         IdResponseErrorsValidate,
		}
	}
	RateLimitedResponse = Result{
		Err:        errors.New("too many requests"),
		StatusCode: http.StatusTooManyRequests,
		Id:         IdResponseErrorsRateLimit,
	}
	MethodNotAllowedResponse = Result{
		Err:        errors.New("only POST requests are supported"),
		StatusCode: http.StatusMethodNotAllowed,
		Id:         IdResponseErrorsMethodNotAllowed,
	}
	FullQueueResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "queue is full"),
			StatusCode: http.StatusServiceUnavailable,
			Id:         IdResponseErrorsFullQueue,
		}
	}
	ServerShuttingDownResponse = func(err error) Result {
		return Result{
			Err:        errors.New("server is shutting down"),
			StatusCode: http.StatusServiceUnavailable,
			Id:         IdResponseErrorsShuttingDown,
		}
	}
)

type ResultID string

type Result struct {
	Id         ResultID
	StatusCode int
	Keyword    string
	Body       interface{}
	Err        error
	Stacktrace string
}

//TODO: move to Context when reworking response handling.
func SendStatus(c *Context, res Result) {
	if res.Err != nil {
		body := map[string]interface{}{"error": res.Err.Error()}
		//TODO: refactor response handling: get rid of additional `error` and just pass in error
		c.SendError(body, body, res.StatusCode)
		return

	}
	if res.Body == nil {
		c.WriteHeader(res.StatusCode)
		return
	}

	c.Send(res.Body, res.StatusCode)
}
