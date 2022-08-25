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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/decoder"
	"github.com/elastic/apm-server/internal/model"
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
		stream io.ReadCloser,
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

		// When the request processing is set to be asynchronous, a few things
		// need to be copied:
		// 1. The context authorizer into a new context, since the current
		//    context is cancelled after the server handles the request.
		// 2. The Request.Body contents. The HTTP server closes the body after
		//    a request has been handled. Replacing Request.Body contents won't
		//    work, because the HTTP server keeps a reference to the body and
		//    closes that.
		// This will result in an async request to perform synchronous work.
		// The amount of bytes copied is minimized by copying the body contents
		// before we attempt to create the de-compressors. It's still possible
		// that a request is uncompressed, in which case, more copying will
		// take place.
		ctx := c.Request.Context()
		if c.Async {
			ctx = auth.CopyAuthorizer(ctx, context.Background())
			b, err := newPooledBodyReader(c.Request.Body)
			if err != nil {
				writeError(c, err)
				return
			}
			c.Request.Body = b // Replace reference with pooledBodyReader.
		}

		reader, err := decoder.CompressedRequestReader(c.Request)
		if err != nil {
			writeError(c, compressedRequestReaderError{err})
			return
		}

		base := requestMetadataFunc(c)
		var result stream.Result
		if err := handler.HandleStream(
			ctx,
			c.Async,
			base,
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

var bodyReaderPool sync.Pool

// pooledBodyReader can be used to consume an io.Reader contents, using a
// bufio.Reader to avoid reading entire body contents into memory at once.
// The main use for this reader is to de-couple Request.Body consumption,
// from the http.Request lifecycle.
// Once the entire buffer's contents has been consumed (io.EOF), the reader
// is closed (reset and put back into the pool).
// pooledBodyReader is not safe for concurrent use.
type pooledBodyReader struct {
	bytes.Buffer
	bufioReader *bufio.Reader
	closed      bool
}

func newPooledBodyReader(src io.Reader) (*pooledBodyReader, error) {
	if b, ok := bodyReaderPool.Get().(*pooledBodyReader); ok {
		b.reset(src)
		if _, err := b.Buffer.ReadFrom(b.bufioReader); err != nil {
			return nil, err
		}
		return b, nil
	}
	var buf bytes.Buffer
	bufioReader := bufio.NewReader(src)
	buf.ReadFrom(bufioReader)
	return &pooledBodyReader{
		bufioReader: bufioReader,
		Buffer:      buf,
	}, nil
}

func (b *pooledBodyReader) Read(p []byte) (n int, err error) {
	n, err = b.Buffer.Read(p)
	if err != nil && !b.closed {
		// Close the pooledBodyReader so it can be returned to the pool after
		// io.EOF is returned (all the contents have been read).
		if errors.Is(err, io.EOF) {
			b.Close()
		}
	}
	return
}

func (b *pooledBodyReader) Close() error {
	if !b.closed {
		b.closed = true
		bodyReaderPool.Put(b)
	}
	return nil
}

func (b *pooledBodyReader) reset(src io.Reader) {
	b.bufioReader.Reset(src)
	b.Buffer.Reset()
	b.closed = false
}
