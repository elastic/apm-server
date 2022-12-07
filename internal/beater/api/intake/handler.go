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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go.elastic.co/apm/v2"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/processor/stream"
	"github.com/elastic/apm-server/internal/publish"
)

const (
	batchSize = 10
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.server")

	errMethodNotAllowed   = errors.New("only POST requests are supported")
	errServerShuttingDown = errors.New("server is shutting down")
	errInvalidContentType = errors.New("invalid content type")
)

// StreamHandler is an interface for handling an Elastic APM agent ND-JSON event
// stream, implemented by processor/stream.
type StreamHandler interface {
	HandleStream(
		ctx context.Context,
		async bool,
		base model.APMEvent,
		stream io.Reader,
		batchSize int,
		processor model.BatchProcessor,
		out *stream.Result,
	) error
}

// RequestMetadataFunc is a function type supplied to Handler for extracting
// metadata from the request. This is used for conditionally injecting the
// source IP address as `client.ip` for RUM.
type RequestMetadataFunc func(*request.Context) model.APMEvent

// Handler returns a request.Handler for managing intake requests for backend and rum events.
func Handler(handler StreamHandler, requestMetadataFunc RequestMetadataFunc, batchProcessor model.BatchProcessor) request.Handler {
	return func(c *request.Context) {
		if err := validateRequest(c); err != nil {
			writeError(c, err)
			return
		}

		// Async can be set by clients to request non-blocking event processing,
		// returning immediately with an error `publish.ErrFull` when it can't be
		// serviced.
		// Async processing has weaker guarantees for the client since any
		// errors while processing the batch cannot be communicated back to the
		// client.
		// Instead, errors are logged by the APM Server.
		async := asyncRequest(c.Request)

		// Create a new detached context when asynchronous processing is set,
		// decoupling the context from its deadline, which will finish when
		// the request is handled. The batch will probably be processed after
		// the request has finished, and it would cause an error if the context
		// is done.
		ctx := c.Request.Context()
		if async {
			ctx = apm.DetachedContext(ctx)
		}

		// If there was an error decoding the body, then it Result.Err
		// will already be set. Reformat the error response.
		if c.Result.Err != nil {
			writeError(c, compressedRequestReaderError{c.Result.Err})
			return
		}

		base := requestMetadataFunc(c)
		var result stream.Result
		if err := handler.HandleStream(
			ctx,
			async,
			base,
			c.Request.Body,
			batchSize,
			batchProcessor,
			&result,
		); err != nil {
			result.Add(err)
		}
		writeStreamResult(c, &result)
	}
}

func validateRequest(c *request.Context) error {
	if c.Request.Method != http.MethodPost {
		return errMethodNotAllowed
	}

	// Content-Type, if specified, must contain "application/x-ndjson". If unspecified, we assume this.
	contentType := c.Request.Header.Get(headers.ContentType)
	if contentType != "" && !strings.Contains(contentType, "application/x-ndjson") {
		return fmt.Errorf("%w: '%s'", errInvalidContentType, contentType)
	}
	return nil
}

func writeError(c *request.Context, err error) {
	var result stream.Result
	result.Add(err)
	writeStreamResult(c, &result)
}

func writeStreamResult(c *request.Context, sr *stream.Result) {
	statusCode := http.StatusAccepted
	id := request.IDResponseValidAccepted
	jsonResult := jsonResult{Accepted: sr.Accepted}
	var errorMessages []string

	if n := len(sr.Errors); n > 0 {
		jsonResult.Errors = make([]jsonError, n)
		errorMessages = make([]string, n)
	}

	for i, err := range sr.Errors {
		errID := request.IDResponseErrorsInternal
		var invalidInput *stream.InvalidInputError
		if errors.As(err, &invalidInput) {
			if invalidInput.TooLarge {
				errID = request.IDResponseErrorsRequestTooLarge
			} else {
				errID = request.IDResponseErrorsValidate
			}
			jsonResult.Errors[i] = jsonError{
				Message:  invalidInput.Message,
				Document: invalidInput.Document,
			}
		} else {
			if errors.As(err, &compressedRequestReaderError{}) {
				errID = request.IDResponseErrorsValidate
			} else {
				switch {
				case errors.Is(err, publish.ErrChannelClosed):
					errID = request.IDResponseErrorsShuttingDown
					err = errServerShuttingDown
				case errors.Is(err, publish.ErrFull):
					errID = request.IDResponseErrorsFullQueue
				case errors.Is(err, errMethodNotAllowed):
					errID = request.IDResponseErrorsMethodNotAllowed
				case errors.Is(err, errInvalidContentType):
					errID = request.IDResponseErrorsValidate
				case errors.Is(err, ratelimit.ErrRateLimitExceeded):
					errID = request.IDResponseErrorsRateLimit
				case errors.Is(err, auth.ErrUnauthorized):
					errID = request.IDResponseErrorsForbidden
				}
			}
			jsonResult.Errors[i] = jsonError{Message: err.Error()}
		}
		errorMessages[i] = jsonResult.Errors[i].Message

		var errStatusCode int
		switch errID {
		case request.IDResponseErrorsMethodNotAllowed:
			// TODO: remove exception case and use StatusMethodNotAllowed (breaking bugfix)
			errStatusCode = http.StatusBadRequest
		case request.IDResponseErrorsRequestTooLarge:
			// TODO: remove exception case and use StatusRequestEntityTooLarge (breaking bugfix)
			errStatusCode = http.StatusBadRequest
		default:
			errStatusCode = request.MapResultIDToStatus[errID].Code
		}
		if errStatusCode > statusCode {
			statusCode = errStatusCode
			id = errID
		}
	}

	var err error
	if len(errorMessages) > 0 {
		err = errors.New(strings.Join(errorMessages, ", "))
	}
	writeResult(c, id, statusCode, &jsonResult, err)
}

func writeResult(c *request.Context, id request.ResultID, statusCode int, result *jsonResult, err error) {
	var body interface{}
	if statusCode >= http.StatusBadRequest {
		// this signals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		c.ResponseWriter.Header().Add(headers.Connection, "Close")
		body = result
	} else if _, ok := c.Request.URL.Query()["verbose"]; ok {
		body = result
	}
	c.Result.Set(id, statusCode, request.MapResultIDToStatus[id].Keyword, body, err)
	c.WriteResult()
}

type compressedRequestReaderError struct {
	error
}

type jsonResult struct {
	Accepted int         `json:"accepted"`
	Errors   []jsonError `json:"errors,omitempty"`
}

type jsonError struct {
	Message  string `json:"message"`
	Document string `json:"document,omitempty"`
}

func asyncRequest(req *http.Request) bool {
	var async bool
	if asyncStr := req.URL.Query().Get("async"); asyncStr != "" {
		async, _ = strconv.ParseBool(asyncStr)
	}
	return async
}
