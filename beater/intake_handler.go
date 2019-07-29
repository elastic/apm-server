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

	"golang.org/x/time/rate"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

type intakeHandler struct {
	requestDecoder  decoder.ReqDecoder
	streamProcessor *stream.Processor
	rlc             *rlCache
}

func (v *intakeHandler) statusCode(sr *stream.Result) (int, *monitoring.Int) {
	var code int
	var ct *monitoring.Int
	highestCode := http.StatusAccepted
	monitoringCt := responseAccepted
	for _, err := range sr.Errors {
		switch err.Type {
		case stream.MethodForbiddenErrType:
			code = http.StatusBadRequest
			ct = methodNotAllowedCounter
		case stream.InputTooLargeErrType:
			code = http.StatusBadRequest
			ct = requestTooLargeCounter
		case stream.InvalidInputErrType:
			code = http.StatusBadRequest
			ct = validateCounter
		case stream.QueueFullErrType:
			code = http.StatusServiceUnavailable
			ct = fullQueueCounter
		case stream.ShuttingDownErrType:
			code = http.StatusServiceUnavailable
			ct = serverShuttingDownCounter
		case stream.RateLimitErrType:
			code = http.StatusTooManyRequests
			ct = rateLimitCounter
		default:
			code = http.StatusInternalServerError
			ct = internalErrorCounter
		}
		if code > highestCode {
			highestCode = code
			monitoringCt = ct
		}
	}
	return highestCode, monitoringCt
}

func (v *intakeHandler) sendResponse(c *request.Context, sr *stream.Result) {
	statusCode, counter := v.statusCode(sr)
	responseCounter.Inc()
	counter.Inc()

	if statusCode == http.StatusAccepted {
		responseSuccesses.Inc()
		if _, ok := c.Request.URL.Query()["verbose"]; ok {
			c.Send(sr, statusCode)
		} else {
			c.WriteHeader(statusCode)
		}
	} else {
		responseErrors.Inc()
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		c.Header().Add(headers.Connection, "Close")
		c.SendError(sr, sr.Error(), statusCode)
	}
}

func (v *intakeHandler) sendError(c *request.Context, err *stream.Error) {
	sr := stream.Result{}
	sr.Add(err)
	v.sendResponse(c, &sr)
}

func (v *intakeHandler) validateRequest(r *http.Request) *stream.Error {
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

func (v *intakeHandler) rateLimit(r *http.Request) (*rate.Limiter, *stream.Error) {
	if rl, ok := v.rlc.getRateLimiter(utility.RemoteAddr(r)); ok {
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

func (v *intakeHandler) bodyReader(r *http.Request) (io.ReadCloser, *stream.Error) {
	reader, err := decoder.CompressedRequestReader(r)
	if err != nil {
		return nil, &stream.Error{
			Type:    stream.InvalidInputErrType,
			Message: err.Error(),
		}
	}
	return reader, nil
}

func (v *intakeHandler) Handle(beaterConfig *Config, report publish.Reporter) Handler {
	return func(c *request.Context) {

		serr := v.validateRequest(c.Request)
		if serr != nil {
			v.sendError(c, serr)
			return
		}

		rl, serr := v.rateLimit(c.Request)
		if serr != nil {
			v.sendError(c, serr)
			return
		}

		reader, serr := v.bodyReader(c.Request)
		if serr != nil {
			v.sendError(c, serr)
			return
		}

		// extract metadata information from the request, like user-agent or remote address
		reqMeta, err := v.requestDecoder(c.Request)
		if err != nil {
			sr := stream.Result{}
			sr.Add(err)
			v.sendResponse(c, &sr)
			return
		}
		res := v.streamProcessor.HandleStream(c.Request.Context(), rl, reqMeta, reader, report)

		v.sendResponse(c, res)
	}
}
