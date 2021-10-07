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
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
)

const Dataset = "firehose"

type record struct {
	Data string `json:"data"`
}

type firehoseLog struct {
	RequestID string   `json:"requestId"`
	Timestamp int64    `json:"timestamp"`
	Records   []record `json:"records"`
}

// RequestMetadataFunc is a function type supplied to Handler for extracting
// Firehose ARN from the request.
type RequestMetadataFunc func(*request.Context) model.APMEvent

// Handler returns a request.Handler for managing firehose requests.
func Handler(requestMetadataFunc RequestMetadataFunc, processor model.BatchProcessor, authenticator *auth.Authenticator) request.Handler {
	handle := func(c *request.Context) (*result, error) {
		accessKey := c.Request.Header.Get("X-Amz-Firehose-Access-Key")
		if accessKey == "" {
			return nil, requestError{
				id:  request.IDResponseErrorsUnauthorized,
				err: errors.New("Access key is required for using /firehose endpoint"),
			}
		}

		details, authorizer, err := authenticator.Authenticate(context.Background(), headers.APIKey, accessKey)
		if err != nil {
			return nil, requestError{
				id:  request.IDResponseErrorsUnauthorized,
				err: errors.New("authentication failed"),
			}
		}

		c.Authentication = details
		c.Request = c.Request.WithContext(auth.ContextWithAuthorizer(c.Request.Context(), authorizer))
		if c.Request.Method != http.MethodPost {
			return nil, requestError{
				id:  request.IDResponseErrorsMethodNotAllowed,
				err: errors.New("only POST requests are supported"),
			}
		}

		var firehose firehoseLog
		err = json.NewDecoder(c.Request.Body).Decode(&firehose)
		if err != nil {
			return nil, err
		}

		// convert firehose log to events
		batch, err := processFirehoseLog(c, firehose, requestMetadataFunc)
		if err != nil {
			return nil, err
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
		// Set required requestId and timestamp to match Firehose HTTP delivery
		// request response format.
		// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
		return &result{RequestID: firehose.RequestID, Timestamp: firehose.Timestamp}, nil
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
			c.Result.StatusCode = 400
		} else {
			c.Result.SetWithBody(request.IDResponseValidAccepted, result)
			c.Result.StatusCode = 200
		}

		// Set response header
		c.Header().Set(headers.ContentType, "application/json")
		c.Write()
	}
}

type result struct {
	ErrorMessage string `json:"errorMessage"`
	RequestID    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
}

type requestError struct {
	id  request.ResultID
	err error
}

func (e requestError) Error() string {
	return e.err.Error()
}

func processFirehoseLog(c *request.Context, firehose firehoseLog, requestMetadataFunc RequestMetadataFunc) (model.Batch, error) {
	var batch model.Batch
	for _, record := range firehose.Records {
		recordDec, err := base64.StdEncoding.DecodeString(record.Data)
		if err != nil {
			return nil, err
		}

		splitLines := strings.Split(string(recordDec), "\n")
		for _, line := range splitLines {
			if line == "" {
				break
			}

			event := requestMetadataFunc(c)
			event.Timestamp = time.Unix(firehose.Timestamp/1000, 0)
			event.Processor = model.LogProcessor
			event.Message = line
			batch = append(batch, event)
		}
	}
	return batch, nil
}
