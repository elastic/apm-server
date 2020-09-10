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

package intake

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.server")
)

// Handler returns a request.Handler for managing intake requests for backend and rum events.
func Handler(processor *stream.Processor, report publish.Reporter) request.Handler {
	return func(c *request.Context) {

		serr := validateRequest(c.Request)
		if serr != nil {
			sendError(c, serr)
			return
		}

		ok := c.RateLimiter == nil || c.RateLimiter.Allow()
		if !ok {
			sendError(c, &stream.Error{
				Type: stream.RateLimitErrType, Message: "rate limit exceeded"})
			return
		}

		reader, serr := bodyReader(c.Request)
		if serr != nil {
			sendError(c, serr)
			return
		}

		metadata := model.Metadata{
			UserAgent: model.UserAgent{Original: c.RequestMetadata.UserAgent},
			Client:    model.Client{IP: c.RequestMetadata.ClientIP},
			System:    model.System{IP: c.RequestMetadata.SystemIP}}
		res := processor.HandleStream(c.Request.Context(), c.RateLimiter, &metadata, reader, report)
		sendResponse(c, res)
	}
}

func sendResponse(c *request.Context, sr *stream.Result) {
	code := http.StatusAccepted
	id := request.IDResponseValidAccepted
	set := func(c int, i request.ResultID) {
		if c > code {
			code = c
			id = i
		}
	}

L:
	for _, err := range sr.Errors {
		switch err.Type {
		case stream.MethodForbiddenErrType:
			// TODO: remove exception case and use StatusMethodNotAllowed (breaking bugfix)
			set(http.StatusBadRequest, request.IDResponseErrorsMethodNotAllowed)
		case stream.InputTooLargeErrType:
			// TODO: remove exception case and use StatusRequestEntityTooLarge (breaking bugfix)
			set(http.StatusBadRequest, request.IDResponseErrorsRequestTooLarge)
		case stream.InvalidInputErrType:
			set(request.MapResultIDToStatus[request.IDResponseErrorsValidate].Code, request.IDResponseErrorsValidate)
		case stream.RateLimitErrType:
			set(request.MapResultIDToStatus[request.IDResponseErrorsRateLimit].Code, request.IDResponseErrorsRateLimit)
		case stream.QueueFullErrType:
			set(request.MapResultIDToStatus[request.IDResponseErrorsFullQueue].Code, request.IDResponseErrorsFullQueue)
			break L
		case stream.ShuttingDownErrType:
			set(request.MapResultIDToStatus[request.IDResponseErrorsShuttingDown].Code, request.IDResponseErrorsShuttingDown)
			break L
		default:
			set(request.MapResultIDToStatus[request.IDResponseErrorsInternal].Code, request.IDResponseErrorsInternal)
		}
	}

	var body interface{}
	if code >= http.StatusBadRequest {
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		c.Header().Add(headers.Connection, "Close")
		body = sr
	} else if _, ok := c.Request.URL.Query()["verbose"]; ok {
		body = sr
	}
	var err error
	if errMsg := sr.Error(); errMsg != "" {
		err = errors.New(errMsg)
	}
	c.Result.Set(id, code, request.MapResultIDToStatus[id].Keyword, body, err)
	c.Write()
}
func sendError(c *request.Context, err *stream.Error) {
	sr := stream.Result{}
	sr.Add(err)
	sendResponse(c, &sr)
}

func validateRequest(r *http.Request) *stream.Error {
	if r.Method != http.MethodPost {
		return &stream.Error{
			Type:    stream.MethodForbiddenErrType,
			Message: "only POST requests are supported",
		}
	}

	if !strings.Contains(r.Header.Get(headers.ContentType), "application/x-ndjson") {
		return &stream.Error{
			Type:    stream.InvalidInputErrType,
			Message: fmt.Sprintf("invalid content type: '%s'", r.Header.Get(headers.ContentType)),
		}
	}
	return nil
}

func bodyReader(r *http.Request) (io.ReadCloser, *stream.Error) {
	reader, err := decoder.CompressedRequestReader(r)
	if err != nil {
		return nil, &stream.Error{
			Type:    stream.InvalidInputErrType,
			Message: err.Error(),
		}
	}
	return reader, nil
}
