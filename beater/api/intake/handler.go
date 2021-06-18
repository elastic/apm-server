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
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
)

const (
	batchSize        = 10
	rateLimitTimeout = time.Second
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.server")

	errMethodNotAllowed   = errors.New("only POST requests are supported")
	errServerShuttingDown = errors.New("server is shutting down")
	errInvalidContentType = errors.New("invalid content type")
	errRateLimitExceeded  = errors.New("rate limit exceeded")
)

// StreamHandler is an interface for handling an Elastic APM agent ND-JSON event
// stream, implemented by processor/stream.
type StreamHandler interface {
	HandleStream(
		ctx context.Context,
		meta *model.Metadata,
		stream io.Reader,
		batchSize int,
		processor model.BatchProcessor,
		out *stream.Result,
	) error
}

// Handler returns a request.Handler for managing intake requests for backend and rum events.
func Handler(handler StreamHandler, batchProcessor model.BatchProcessor) request.Handler {
	return func(c *request.Context) {
		if err := validateRequest(c); err != nil {
			writeError(c, err)
			return
		}

		if c.RateLimiter != nil {
			// Apply rate limiting after reading but before processing any events.
			batchProcessor = modelprocessor.Chained{
				rateLimitBatchProcessor(c.RateLimiter, batchSize),
				batchProcessor,
			}
		}

		reader, err := decoder.CompressedRequestReader(c.Request)
		if err != nil {
			writeError(c, compressedRequestReaderError{err})
			return
		}

		metadata := model.Metadata{
			UserAgent: model.UserAgent{Original: c.RequestMetadata.UserAgent},
			Client:    model.Client{IP: c.RequestMetadata.ClientIP},
			System:    model.System{IP: c.RequestMetadata.SystemIP},
		}

		var result stream.Result
		if err := handler.HandleStream(
			c.Request.Context(),
			&metadata,
			reader,
			batchSize,
			batchProcessor,
			&result,
		); err != nil {
			result.Add(err)
		}
		writeStreamResult(c, &result)
	}
}

func rateLimitBatchProcessor(limiter *rate.Limiter, batchSize int) model.ProcessBatchFunc {
	return func(ctx context.Context, _ *model.Batch) error {
		return rateLimitBatch(ctx, limiter, batchSize)
	}
}

// rateLimitBatch waits up to one second for the rate limiter to allow
// batchSize events to be read and processed.
func rateLimitBatch(ctx context.Context, limiter *rate.Limiter, batchSize int) error {
	ctx, cancel := context.WithTimeout(ctx, rateLimitTimeout)
	defer cancel()
	if err := limiter.WaitN(ctx, batchSize); err != nil {
		return errRateLimitExceeded
	}
	return nil
}

func validateRequest(c *request.Context) error {
	if c.Request.Method != http.MethodPost {
		return errMethodNotAllowed
	}
	if contentType := c.Request.Header.Get(headers.ContentType); !strings.Contains(contentType, "application/x-ndjson") {
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
				case errors.Is(err, errRateLimitExceeded):
					errID = request.IDResponseErrorsRateLimit
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
		c.Header().Add(headers.Connection, "Close")
		body = result
	} else if _, ok := c.Request.URL.Query()["verbose"]; ok {
		body = result
	}
	c.Result.Set(id, statusCode, request.MapResultIDToStatus[id].Keyword, body, err)
	c.Write()
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
