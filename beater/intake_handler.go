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

package beater

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

var (
	mu sync.Mutex

	serverMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counter       = func(s request.ResultID) *monitoring.Int {
		return monitoring.NewInt(serverMetrics, string(s))
	}

	resultIDToCounter = map[request.ResultID]*monitoring.Int{}
)

func intakeResultIDToMonitoringInt(name request.ResultID) *monitoring.Int {
	if i, ok := resultIDToCounter[name]; ok {
		return i
	}

	mu.Lock()
	defer mu.Unlock()
	if i, ok := monitoring.Get(string(name)).(*monitoring.Int); ok {
		resultIDToCounter[name] = i
		return i
	}
	ct := counter(name)
	resultIDToCounter[name] = ct
	return ct
}

func statusCode(sr *stream.Result) (int, request.ResultID) {
	var code int
	var id request.ResultID
	highestCode := http.StatusAccepted
	monitoringID := request.IDResponseValidAccepted
	for _, err := range sr.Errors {
		switch err.Type {
		case stream.MethodForbiddenErrType:
			code = http.StatusBadRequest
			id = request.IDResponseErrorsMethodNotAllowed
		case stream.InputTooLargeErrType:
			code = http.StatusBadRequest
			id = request.IDResponseErrorsRequestTooLarge
		case stream.InvalidInputErrType:
			code = http.StatusBadRequest
			id = request.IDResponseErrorsValidate
		case stream.QueueFullErrType:
			code = http.StatusServiceUnavailable
			id = request.IDResponseErrorsFullQueue
		case stream.ShuttingDownErrType:
			code = http.StatusServiceUnavailable
			id = request.IDResponseErrorsShuttingDown
		case stream.RateLimitErrType:
			code = http.StatusTooManyRequests
			id = request.IDResponseErrorsRateLimit
		default:
			code = http.StatusInternalServerError
			id = request.IDResponseErrorsInternal
		}
		if code > highestCode {
			highestCode = code
			monitoringID = id
		}
	}
	return highestCode, monitoringID
}

func sendResponse(c *request.Context, sr *stream.Result) {
	statusCode, id := statusCode(sr)
	c.MonitoringID = id
	if statusCode == http.StatusAccepted {
		if _, ok := c.Request.URL.Query()["verbose"]; ok {
			c.Send(sr, statusCode)
		} else {
			c.WriteHeader(statusCode)
		}
	} else {
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		c.Header().Add(headers.Connection, "Close")
		c.SendError(sr, sr.Error(), statusCode)
	}
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

func rateLimit(r *http.Request, rlc *rlCache) (*rate.Limiter, *stream.Error) {
	if rl, ok := rlc.getRateLimiter(utility.RemoteAddr(r)); ok {
		if !rl.Allow() {
			return nil, &stream.Error{
				Type:    stream.RateLimitErrType,
				Message: "rate limit exceeded",
			}
		}
		return rl, nil
	}
	return nil, nil
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

func newIntakeHandler(dec decoder.ReqDecoder, processor *stream.Processor, rlc *rlCache, report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		serr := validateRequest(c.Request)
		if serr != nil {
			sendError(c, serr)
			return
		}

		rl, serr := rateLimit(c.Request, rlc)
		if serr != nil {
			sendError(c, serr)
			return
		}

		reader, serr := bodyReader(c.Request)
		if serr != nil {
			sendError(c, serr)
			return
		}

		// extract metadata information from the request, like user-agent or remote address
		reqMeta, err := dec(c.Request)
		if err != nil {
			sr := stream.Result{}
			sr.Add(err)
			sendResponse(c, &sr)
			return
		}
		res := processor.HandleStream(c.Request.Context(), rl, reqMeta, reader, report)

		sendResponse(c, res)
	}
}
