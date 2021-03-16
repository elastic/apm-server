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

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/pkg/errors"
)

const (

	// IDUnset identifies requests not covered by other IDs
	IDUnset ResultID = "unset"

	// IDRequestCount identifies all requests
	IDRequestCount ResultID = "request.count"
	// IDResponseCount identifies all responses
	IDResponseCount ResultID = "response.count"
	// IDResponseErrorsCount identifies all non successful responses
	IDResponseErrorsCount ResultID = "response.errors.count"
	// IDResponseValidCount identifies all successful responses
	IDResponseValidCount ResultID = "response.valid.count"
	// IDEventReceivedCount identifies amount of received events
	IDEventReceivedCount ResultID = "event.received.count"
	// IDEventDroppedCount identifies amount of dropped events
	IDEventDroppedCount ResultID = "event.dropped.count"

	// IDResponseValidNotModified identifies all successful responses without a modified body
	IDResponseValidNotModified ResultID = "response.valid.notmodified"
	// IDResponseValidOK identifies responses with status code 200
	IDResponseValidOK ResultID = "response.valid.ok"
	// IDResponseValidAccepted identifies responses with status code 202
	IDResponseValidAccepted ResultID = "response.valid.accepted"

	// IDResponseErrorsForbidden identifies responses for forbidden requests
	IDResponseErrorsForbidden ResultID = "response.errors.forbidden"
	// IDResponseErrorsUnauthorized identifies responses for unauthorized requests
	IDResponseErrorsUnauthorized ResultID = "response.errors.unauthorized"
	// IDResponseErrorsNotFound identifies responses where route was not found
	IDResponseErrorsNotFound ResultID = "response.errors.notfound"
	// IDResponseErrorsInvalidQuery identifies responses with invalid query sent
	IDResponseErrorsInvalidQuery ResultID = "response.errors.invalidquery"
	// IDResponseErrorsRequestTooLarge identifies responses for too large requests
	IDResponseErrorsRequestTooLarge ResultID = "response.errors.toolarge"
	// IDResponseErrorsDecode identifies responses for requests that could not be decoded
	IDResponseErrorsDecode ResultID = "response.errors.decode"
	// IDResponseErrorsValidate identifies responses for invalid requests
	IDResponseErrorsValidate ResultID = "response.errors.validate"
	// IDResponseErrorsRateLimit identifies responses for rate limited requests
	IDResponseErrorsRateLimit ResultID = "response.errors.ratelimit"
	// IDResponseErrorsTimeout identifies responses for timed out requests
	IDResponseErrorsTimeout ResultID = "response.errors.timeout"
	// IDResponseErrorsMethodNotAllowed identifies responses for requests using a forbidden method
	IDResponseErrorsMethodNotAllowed ResultID = "response.errors.method"
	// IDResponseErrorsFullQueue identifies responses when internal queue was full
	IDResponseErrorsFullQueue ResultID = "response.errors.queue"
	// IDResponseErrorsShuttingDown identifies responses requests occuring after channel was closed
	IDResponseErrorsShuttingDown ResultID = "response.errors.closed"
	// IDResponseErrorsServiceUnavailable identifies responses where service was unavailable
	IDResponseErrorsServiceUnavailable ResultID = "response.errors.unavailable"
	// IDResponseErrorsInternal identifies responses where internal errors occured
	IDResponseErrorsInternal ResultID = "response.errors.internal"
	// IDResponseErrorsServiceUnavailable identifies responses where resource is unavailable
)

var (
	// MapResultIDToStatus takes a ResultID and maps it to a status
	MapResultIDToStatus = map[ResultID]Status{
		IDResponseValidOK:                  {Code: http.StatusOK, Keyword: "request ok"},
		IDResponseValidAccepted:            {Code: http.StatusAccepted, Keyword: "request accepted"},
		IDResponseValidNotModified:         {Code: http.StatusNotModified, Keyword: "not modified"},
		IDResponseErrorsForbidden:          {Code: http.StatusForbidden, Keyword: "forbidden request"},
		IDResponseErrorsUnauthorized:       {Code: http.StatusUnauthorized, Keyword: "unauthorized"},
		IDResponseErrorsNotFound:           {Code: http.StatusNotFound, Keyword: "404 page not found"},
		IDResponseErrorsRequestTooLarge:    {Code: http.StatusRequestEntityTooLarge, Keyword: "request body too large"},
		IDResponseErrorsInvalidQuery:       {Code: http.StatusBadRequest, Keyword: "invalid query"},
		IDResponseErrorsDecode:             {Code: http.StatusBadRequest, Keyword: "data decoding error"},
		IDResponseErrorsValidate:           {Code: http.StatusBadRequest, Keyword: "data validation error"},
		IDResponseErrorsMethodNotAllowed:   {Code: http.StatusMethodNotAllowed, Keyword: "method not supported"},
		IDResponseErrorsRateLimit:          {Code: http.StatusTooManyRequests, Keyword: "too many requests"},
		IDResponseErrorsTimeout:            {Code: http.StatusServiceUnavailable, Keyword: "request timed out"},
		IDResponseErrorsFullQueue:          {Code: http.StatusServiceUnavailable, Keyword: "queue is full"},
		IDResponseErrorsShuttingDown:       {Code: http.StatusServiceUnavailable, Keyword: "server is shutting down"},
		IDResponseErrorsServiceUnavailable: {Code: http.StatusServiceUnavailable, Keyword: "service unavailable"},
		IDResponseErrorsInternal:           {Code: http.StatusInternalServerError, Keyword: "internal error"},
	}

	// DefaultResultIDs is a list of the default result IDs used by the package.
	DefaultResultIDs = []ResultID{IDRequestCount, IDResponseCount, IDResponseErrorsCount, IDResponseValidCount}
)

// ResultID unique string identifying a requests Result
type ResultID string

// Status holds statuscode and keyword information
type Status struct {
	Code    int
	Keyword string
}

// Result holds information about a processed request
type Result struct {
	ID         ResultID
	StatusCode int
	Keyword    string
	Body       interface{}
	Err        error
	Stacktrace string
}

// DefaultMonitoringMapForRegistry returns map matching resultIDs to monitoring counters for given registry.
func DefaultMonitoringMapForRegistry(r *monitoring.Registry) map[ResultID]*monitoring.Int {
	ids := append(DefaultResultIDs, IDUnset)
	for id := range MapResultIDToStatus {
		ids = append(ids, id)
	}
	return MonitoringMapForRegistry(r, ids)
}

// MonitoringMapForRegistry returns map matching resultIDs to monitoring counters for given registry and keys
func MonitoringMapForRegistry(r *monitoring.Registry, ids []ResultID) map[ResultID]*monitoring.Int {
	m := map[ResultID]*monitoring.Int{}
	counter := func(s ResultID) *monitoring.Int {
		return monitoring.NewInt(r, string(s))
	}
	for _, id := range ids {
		m[id] = counter(id)
	}
	return m
}

// Reset sets result to it's empty values
func (r *Result) Reset() {
	r.ID = IDUnset
	r.StatusCode = http.StatusOK
	r.Keyword = ""
	r.Body = nil
	r.Err = nil
	r.Stacktrace = ""
}

// Failure returns a bool indicating whether it is describing a successful result or not
func (r *Result) Failure() bool {
	return r.StatusCode >= http.StatusBadRequest
}

// SetDefault derives information about the result solely from the ID.
func (r *Result) SetDefault(id ResultID) {
	r.set(id, nil, nil)
}

// SetWithError derives information about the result from the given ID and the error.
// The body is derived from the error in case the result describes a failure.
func (r *Result) SetWithError(id ResultID, err error) {
	r.set(id, nil, err)
}

// SetWithBody derives information about the result from the given ID. The body is set to the passed value.
func (r *Result) SetWithBody(id ResultID, body interface{}) {
	r.set(id, body, nil)
}

// SetDeniedAuthorization sets the result when authorization is denied
func (r *Result) SetDeniedAuthorization(err error) {
	if err != nil {
		id := IDResponseErrorsServiceUnavailable
		status := MapResultIDToStatus[id]
		r.Set(id, status.Code, status.Keyword, status.Keyword, err)
	}
	r.SetDefault(IDResponseErrorsUnauthorized)
}

// Set allows for the most flexibility in setting a result's properties.
// The error and body information are derived from the given parameters.
func (r *Result) Set(id ResultID, statusCode int, keyword string, body interface{}, err error) {
	if r == nil {
		return
	}
	r.ID = id
	r.StatusCode = statusCode
	r.Keyword = keyword
	r.Body = body
	r.Err = err

	if r.Failure() {
		if err == nil {
			r.Err = errors.New(keyword)
		}
		if r.Body == nil {
			r.Body = r.Err.Error()
		}
	}
}

func (r *Result) set(id ResultID, body interface{}, err error) {
	if r == nil {
		return
	}

	var statusCode int
	var keyword string
	if status, ok := MapResultIDToStatus[id]; ok {
		statusCode = status.Code
		keyword = status.Keyword
	} else {
		statusCode = MapResultIDToStatus[IDResponseErrorsInternal].Code
		keyword = MapResultIDToStatus[IDResponseErrorsInternal].Keyword
	}
	err = errors.Wrap(err, keyword)
	r.Set(id, statusCode, keyword, body, err)
}
