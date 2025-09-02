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
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/apm-data/input/elasticapm"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/publish"
)

const (
	batchSize = 10
)

var (
	errMethodNotAllowed   = errors.New("only POST requests are supported")
	errServerShuttingDown = errors.New("server is shutting down")
	errInvalidContentType = errors.New("invalid content type")
)

// RequestMetadataFunc is a function type supplied to Handler for extracting
// metadata from the request. This is used for conditionally injecting the
// source IP address as `client.ip` for RUM.
type RequestMetadataFunc func(*request.Context) *modelpb.APMEvent

// Handler returns a request.Handler for managing intake requests for backend and rum events.
func Handler(mp metric.MeterProvider, tp trace.TracerProvider, handler elasticapm.StreamHandler, requestMetadataFunc RequestMetadataFunc, batchProcessor modelpb.BatchProcessor) request.Handler {
	meter := mp.Meter("github.com/elastic/apm-server/internal/beater/api/intake")
	eventsAccepted, _ := meter.Int64Counter("apm-server.processor.stream.accepted")
	eventsInvalid, _ := meter.Int64Counter("apm-server.processor.stream.errors.invalid")
	eventsTooLarge, _ := meter.Int64Counter("apm-server.processor.stream.errors.toolarge")

	batchProcessor = modelprocessor.NewTracer("intake.ProcessBatch", batchProcessor, modelprocessor.WithTracerProvider(tp))

	return func(c *request.Context) {
		if err := validateRequest(c); err != nil {
			writeError(c, err)
			return
		}

		// If there was an error decoding the body, then it Result.Err
		// will already be set. Reformat the error response.
		if c.Result.Err != nil {
			writeError(c, compressedRequestReaderError{c.Result.Err})
			return
		}

		var result elasticapm.Result
		err := handler.HandleStream(
			c.Request.Context(),
			requestMetadataFunc(c),
			c.Request.Body,
			batchSize,
			batchProcessor,
			&result,
		)
		eventsAccepted.Add(context.Background(), int64(result.Accepted))
		eventsInvalid.Add(context.Background(), int64(result.Invalid))
		eventsTooLarge.Add(context.Background(), int64(result.TooLarge))
		writeStreamResult(c, result, err)
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
	writeStreamResult(c, elasticapm.Result{}, err)
}

func writeStreamResult(c *request.Context, streamResult elasticapm.Result, streamErr error) {
	statusCode := http.StatusAccepted
	id := request.IDResponseValidAccepted
	jsonResult := jsonResult{Accepted: streamResult.Accepted}
	var errorMessages []string

	if n := len(streamResult.Errors); n > 0 {
		if streamErr != nil {
			n++
		}
		jsonResult.Errors = make([]jsonError, 0, n)
		errorMessages = make([]string, 0, n)
	}

	processError := func(err error) {
		errID, jsonErr := processStreamError(err)
		errStatusCode := errStatusCode(errID)
		jsonResult.Errors = append(jsonResult.Errors, jsonErr)
		errorMessages = append(errorMessages, jsonErr.Message)
		if errStatusCode > statusCode {
			statusCode = errStatusCode
			id = errID
		}
	}
	for _, err := range streamResult.Errors {
		processError(err)
	}
	if streamErr != nil {
		processError(streamErr)
	}

	var err error
	if len(errorMessages) > 0 {
		err = errors.New(strings.Join(errorMessages, ", "))
	}
	writeResult(c, id, statusCode, &jsonResult, err)
}

func processStreamError(err error) (request.ResultID, jsonError) {
	errID := request.IDResponseErrorsInternal

	var invalidInput *elasticapm.InvalidInputError
	if errors.As(err, &invalidInput) {
		if invalidInput.TooLarge {
			errID = request.IDResponseErrorsRequestTooLarge
		} else {
			errID = request.IDResponseErrorsValidate
		}
		return errID, jsonError{
			Message:  invalidInput.Message,
			Document: invalidInput.Document,
		}
	}

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
	return errID, jsonError{Message: err.Error()}
}

func errStatusCode(errID request.ResultID) int {
	switch errID {
	case request.IDResponseErrorsMethodNotAllowed:
		// TODO: remove exception case and use StatusMethodNotAllowed (breaking bugfix)
		return http.StatusBadRequest
	case request.IDResponseErrorsRequestTooLarge:
		// TODO: remove exception case and use StatusRequestEntityTooLarge (breaking bugfix)
		return http.StatusBadRequest
	default:
		return request.MapResultIDToStatus[errID].Code
	}
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
