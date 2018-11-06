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

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

type v2Handler struct {
	requestDecoder  decoder.ReqDecoder
	streamProcessor *stream.StreamProcessor
	rlc             *rlCache
}

func (v *v2Handler) statusCode(sr *stream.Result) (int, *monitoring.Int) {
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

func (v *v2Handler) sendResponse(logger *logp.Logger, w http.ResponseWriter, r *http.Request, sr *stream.Result) {
	statusCode, counter := v.statusCode(sr)
	responseCounter.Inc()
	counter.Inc()

	if statusCode != http.StatusAccepted {
		responseErrors.Inc()
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		w.Header().Add("Connection", "Close")

		if acceptsJSON(r) {
			sendJSON(w, sr, statusCode)
		} else {
			sendPlain(w, sr.String(), statusCode)
		}
		logger.Infow("error handling request", "error", sr.String())
	} else {
		w.WriteHeader(statusCode)
		responseSuccesses.Inc()
	}
}

func (v *v2Handler) sendError(logger *logp.Logger, w http.ResponseWriter, r *http.Request, err *stream.Error) {
	sr := stream.Result{}
	sr.Add(err)
	v.sendResponse(logger, w, r, &sr)
	return
}

func (v *v2Handler) validateRequest(r *http.Request) *stream.Error {
	if r.Method != "POST" {
		return &stream.Error{
			Type:    stream.MethodForbiddenErrType,
			Message: "only POST requests are supported",
		}
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/x-ndjson") {
		return &stream.Error{
			Type:    stream.InvalidInputErrType,
			Message: fmt.Sprintf("invalid content type: '%s'", r.Header.Get("Content-Type")),
		}
	}
	return nil
}

func (v *v2Handler) rateLimit(r *http.Request) (*rate.Limiter, *stream.Error) {
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

func (v *v2Handler) bodyReader(r *http.Request) (io.ReadCloser, *stream.Error) {
	reader, err := decoder.CompressedRequestReader(r)
	if err != nil {
		return nil, &stream.Error{
			Type:    stream.InvalidInputErrType,
			Message: err.Error(),
		}
	}
	return reader, nil
}

func (v *v2Handler) Handle(beaterConfig *Config, report publish.Reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := requestLogger(r)

		serr := v.validateRequest(r)
		if serr != nil {
			v.sendError(logger, w, r, serr)
			return
		}

		rl, serr := v.rateLimit(r)
		if serr != nil {
			v.sendError(logger, w, r, serr)
			return
		}

		reader, serr := v.bodyReader(r)
		if serr != nil {
			v.sendError(logger, w, r, serr)
			return
		}

		// extract metadata information from the request, like user-agent or remote address
		reqMeta, err := v.requestDecoder(r)
		if err != nil {
			sr := stream.Result{}
			sr.Add(err)
			v.sendResponse(logger, w, r, &sr)
			return
		}
		res := v.streamProcessor.HandleStream(r.Context(), rl, reqMeta, reader, report)

		v.sendResponse(logger, w, r, res)
	})
}
