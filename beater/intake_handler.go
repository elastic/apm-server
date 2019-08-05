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

	"github.com/pkg/errors"

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

// IntakeResultIDToMonitoringInt takes a request.ResultID and maps it to a monitoring counter. If the ID is UnsetID,
// nil is returned.
func IntakeResultIDToMonitoringInt(name request.ResultID) *monitoring.Int {
	if i, ok := resultIDToCounter[name]; ok {
		return i
	}

	//TODO: remove this to also count unset IDs as indicator that some ID has not been set
	if name == request.IDUnset {
		return nil
	}

	if i, ok := monitoring.Get(string(name)).(*monitoring.Int); ok {
		mu.Lock()
		defer mu.Unlock()
		resultIDToCounter[name] = i
		return i
	}

	mu.Lock()
	defer mu.Unlock()
	ct := counter(name)
	resultIDToCounter[name] = ct
	return ct
}

// IntakeHandler returns a request.Handler for managing intake requests for backend and rum events.
func IntakeHandler(dec decoder.ReqDecoder, processor *stream.Processor, rlc *rlCache, report publish.Reporter) request.Handler {
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

func sendResponse(c *request.Context, sr *stream.Result) {
	code := http.StatusAccepted
	id := request.IDResponseValidAccepted
	err := errors.New(sr.Error())
	var body interface{}

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

	if code >= http.StatusBadRequest {
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		c.Header().Add(headers.Connection, "Close")
		body = sr
	} else if _, ok := c.Request.URL.Query()["verbose"]; ok {
		body = sr
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
