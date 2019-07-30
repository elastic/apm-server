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

//Ids can be used for monitoring counters
const (

	//IDRequestCount identifies all requests
	IDRequestCount ResultID = "request.count"
	//IDResponseCount identifies all responses
	IDResponseCount ResultID = "response.count"
	//IDResponseErrorsCount identifies all non successful responses
	IDResponseErrorsCount ResultID = "response.errors.count"
	//IDResponseValidCount identifies all successful responses
	IDResponseValidCount ResultID = "response.valid.count"

	//IDResponseValidOK identifies responses with status code 200
	IDResponseValidOK ResultID = "response.valid.ok"
	//IDResponseValidAccepted identifies responses with status code 202
	IDResponseValidAccepted ResultID = "response.valid.accepted"

	//IDResponseErrorsForbidden identifies responses for forbidden requests
	IDResponseErrorsForbidden ResultID = "response.errors.forbidden"
	//IDResponseErrorsUnauthorized identifies responses for unauthorized requests
	IDResponseErrorsUnauthorized ResultID = "response.errors.unauthorized"
	//IDResponseErrorsRequestTooLarge identifies responses for too large requests
	IDResponseErrorsRequestTooLarge ResultID = "response.errors.toolarge"
	//IDResponseErrorsDecode identifies responses for requests that could not be decoded
	IDResponseErrorsDecode ResultID = "response.errors.decode"
	//IDResponseErrorsValidate identifies responses for invalid requests
	IDResponseErrorsValidate ResultID = "response.errors.validate"
	//IDResponseErrorsRateLimit identifies responses for rate limited requests
	IDResponseErrorsRateLimit ResultID = "response.errors.ratelimit"
	//IDResponseErrorsMethodNotAllowed identifies responses for requests using a forbidden method
	IDResponseErrorsMethodNotAllowed ResultID = "response.errors.method"
	//IDResponseErrorsFullQueue identifies responses when internal queue was full
	IDResponseErrorsFullQueue ResultID = "response.errors.queue"
	//IDResponseErrorsShuttingDown identifies responses requests occuring after channel was closed
	IDResponseErrorsShuttingDown ResultID = "response.errors.closed"
	//IDResponseErrorsInternal identifies responses where internal errors occured
	IDResponseErrorsInternal ResultID = "response.errors.internal"
)

var (
	//OKResponse represents status 200
	OKResponse = Result{
		StatusCode: http.StatusOK,
		ID:         IDResponseValidOK,
	}
	//AcceptedResponse represents status 200
	AcceptedResponse = Result{
		StatusCode: http.StatusAccepted,
		ID:         IDResponseValidAccepted,
	}
	//InternalErrorResponse represents status 500
	InternalErrorResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "internal error"),
			StatusCode: http.StatusInternalServerError,
			ID:         IDResponseErrorsInternal,
		}
	}
	//ForbiddenResponse represents status 403
	ForbiddenResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "forbidden request"),
			StatusCode: http.StatusForbidden,
			ID:         IDResponseErrorsForbidden,
		}
	}
	//UnauthorizedResponse represents status 401
	UnauthorizedResponse = Result{
		Err:        errors.New("invalid token"),
		StatusCode: http.StatusUnauthorized,
		ID:         IDResponseErrorsUnauthorized,
	}
	//RequestTooLargeResponse represents status 413
	RequestTooLargeResponse = Result{
		Err:        errors.New("request body too large"),
		StatusCode: http.StatusRequestEntityTooLarge,
		ID:         IDResponseErrorsRequestTooLarge,
	}
	//CannotDecodeResponse represents status 400 because of decoding errors
	CannotDecodeResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "data decoding error"),
			StatusCode: http.StatusBadRequest,
			ID:         IDResponseErrorsDecode,
		}
	}
	//CannotValidateResponse represents status 400 because of validation errors
	CannotValidateResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "data validation error"),
			StatusCode: http.StatusBadRequest,
			ID:         IDResponseErrorsValidate,
		}
	}
	//RateLimitedResponse represents status 429
	RateLimitedResponse = Result{
		Err:        errors.New("too many requests"),
		StatusCode: http.StatusTooManyRequests,
		ID:         IDResponseErrorsRateLimit,
	}
	//MethodNotAllowedResponse represents status 405
	MethodNotAllowedResponse = Result{
		Err:        errors.New("only POST requests are supported"),
		StatusCode: http.StatusMethodNotAllowed,
		ID:         IDResponseErrorsMethodNotAllowed,
	}
	//FullQueueResponse represents status 503 because internal memory queue is full
	FullQueueResponse = func(err error) Result {
		return Result{
			Err:        errors.Wrap(err, "queue is full"),
			StatusCode: http.StatusServiceUnavailable,
			ID:         IDResponseErrorsFullQueue,
		}
	}
	//ServerShuttingDownResponse represents status 503 because server is shutting down
	ServerShuttingDownResponse = func(err error) Result {
		return Result{
			Err:        errors.New("server is shutting down"),
			StatusCode: http.StatusServiceUnavailable,
			ID:         IDResponseErrorsShuttingDown,
		}
	}
)

//ResultID unique string identifying a requests Result
type ResultID string

//Result holds information about a processed request
type Result struct {
	ID         ResultID
	StatusCode int
	Keyword    string
	Body       interface{}
	Err        error
	Stacktrace string
}

//SendStatus initiates response writing for a given context
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
