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

package firehose

import (
	b64 "encoding/base64"
	"encoding/json"
	"github.com/elastic/apm-server/beater/headers"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
)

// RequestMetadataFunc is a function type supplied to Handler for extracting
// metadata from the request. This is used for conditionally injecting the
// source IP address as `client.ip` for RUM.
type RequestMetadataFunc func(*request.Context) model.APMEvent

type Record struct {
	Data string `json:"data"`
}

type FirehoseLog struct {
	RequestID string   `json:"requestId"`
	Timestamp int64    `json:"timestamp"`
	Records   []Record `json:"records"`
}

// Handler returns a request.Handler for managing firehose requests.
func Handler(requestMetadataFunc RequestMetadataFunc, processor model.BatchProcessor) request.Handler {
	handle := func(c *request.Context) (*result, error) {
		if c.Request.Method != http.MethodPost {
			return nil, requestError{
				id:  request.IDResponseErrorsMethodNotAllowed,
				err: errors.New("only POST requests are supported"),
			}
		}

		baseEvent := requestMetadataFunc(c)

		var firehose FirehoseLog
		err := json.NewDecoder(c.Request.Body).Decode(&firehose)
		if err != nil {
			return nil, err
		}

		var batch model.Batch
		for _, record := range firehose.Records {
			recordDec, err := b64.StdEncoding.DecodeString(record.Data)
			if err != nil {
				return nil, err
			}

			splitLines := strings.Split(string(recordDec), "\n")
			for _, line := range splitLines {
				if line == "" {
					break
				}

				event := baseEvent
				event.Timestamp = time.Unix(firehose.Timestamp/1000, 0)
				event.Processor = model.FirehoseProcessor
				event.FirehoseLog = &model.Firehose{
					Message: line,
				}
				batch = append(batch, event)
			}
		}

		if err := processor.ProcessBatch(c.Request.Context(), &batch); err != nil {
			switch err {
			case publish.ErrChannelClosed:
				return nil, requestError{
					id:  request.IDResponseErrorsShuttingDown,
					err: errors.New("server is shutting down"),
				}
			case publish.ErrFull:
				return nil, requestError{
					id:  request.IDResponseErrorsFullQueue,
					err: err,
				}
			}
			return nil, err
		}
		return &result{Accepted: len(batch)}, nil
	}

	return func(c *request.Context) {
		result, err := handle(c)
		if err != nil {
			switch err := err.(type) {
			case requestError:
				c.Result.SetWithError(err.id, err)
			default:
				c.Result.SetWithError(request.IDResponseErrorsInternal, err)
			}
		} else {
			c.Result.SetWithBody(request.IDResponseValidAccepted, result)
		}

		// Is this the right way to add response header?
		// Error from Firehose:
		// The response received from the endpoint is invalid. See
		// Troubleshooting HTTP Endpoints in the Firehose documentation for more
		// information. Reason:. Response for request db0a05b1-be46-43e2-93fd-f9
		// must contain a 'content-type: application/json' header. Raw response
		// received: 202: {\"accepted\":54}"
		c.Header().Set(headers.ContentType, "application/json")
		c.Header().Set("content-type", "application/json")
		c.Write()
	}
}

type result struct {
	Accepted int `json:"accepted"`
}

type requestError struct {
	id  request.ResultID
	err error
}

func (e requestError) Error() string {
	return e.err.Error()
}
