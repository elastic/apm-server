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

package stream

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/utility"

	"github.com/elastic/apm-server/model"
	errorm "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/transform"
)

func validMetadata() string {
	return `{"metadata": {"service": {"name": "myservice", "agent": {"name": "test", "version": "1.0"}}}}`
}

func TestV2Handler(t *testing.T) {
	var transformables []transform.Transformable
	var reportedTCtx *transform.Context
	report := func(ctx context.Context, p publish.PendingReq) error {
		transformables = append(transformables, p.Transformables...)
		reportedTCtx = p.Tcontext
		return nil
	}

	tx1 := "tx1"
	spanHexId, traceId := "0147258369abcdef", "abcdefabcdef01234567890123456789"

	timestamp, err := time.Parse(time.RFC3339, "2018-01-01T10:00:00Z")
	assert.NoError(t, err)
	reqTimestamp, err := time.Parse(time.RFC3339, "2018-01-02T10:00:00Z")
	assert.NoError(t, err)

	transactionId := "fedcba0123456789"

	for idx, test := range []struct {
		body         string
		contentType  string
		err          *Result
		expectedCode int
		reported     []transform.Transformable
	}{
		{
			body: strings.Join([]string{
				validMetadata(),
				`{"invalid json"}`,
			}, "\n"),
			err: &Result{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_INVALID_JSON": errorDetails{
						Count:   1,
						Message: "invalid JSON",
						Documents: []*ValidationError{
							{
								Error:          "data read error: invalid character '}' after object key",
								OffendingEvent: `{"invalid json"}`,
							},
						},
					},
				},
				Accepted: 0,
				Dropped:  0,
				Invalid:  1,
			},
			expectedCode: 400,
			reported:     []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				`{"transaction": {"invalid": "metadata"}}`, // invalid metadata
				`{"transaction": {"invalid": "metadata"}}`,
			}, "\n"),
			expectedCode: 400,
			err: &Result{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_SCHEMA_VALIDATION": errorDetails{
						Count:   1,
						Message: "validation error",
						Documents: []*ValidationError{
							{
								Error:          "did not recognize object type",
								OffendingEvent: "{\"transaction\": {\"invalid\": \"metadata\"}}\n",
							},
						},
					},
				},
			},
			reported: []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				`{"metadata": {}}`,
				`{"span": {}}`,
			}, "\n"),
			expectedCode: 400,
			err: &Result{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_SCHEMA_VALIDATION": errorDetails{
						Count:   1,
						Message: "validation error",
						Documents: []*ValidationError{
							{
								Error:          "Problem validating JSON document against schema: I[#] S[#] doesn't validate with \"metadata#\"\n  I[#] S[#/required] missing properties: \"service\"",
								OffendingEvent: "{\"metadata\": {}}\n",
							},
						},
					},
				},
			},
			reported: []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				validMetadata(),
				`{"transaction": {"name": "tx1", "id": "9876543210abcdef", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z", "trace_id": "abcdefabcdef01234567890123456789"}}`,
				`{"span": {"name": "sp1", "duration": 20, "start": 10, "type": "db", "timestamp": "2018-01-01T10:00:00Z", "id": "0147258369abcdef","trace_id": "abcdefabcdef01234567890123456789",  "transaction_id": "fedcba0123456789", "stacktrace": [{"filename": "file.js", "lineno": 10}, {"filename": "file2.js", "lineno": 11}]}}`,
				`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
				`{"error": {"exception": {"message": "hello world!"}}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: http.StatusAccepted,
			reported: []transform.Transformable{
				&transaction.Event{Name: &tx1, Id: "9876543210abcdef", Duration: 12, Type: "request", Timestamp: timestamp, TraceId: &traceId},
				&span.Event{Name: "sp1", Duration: 20.0, Start: 10, Type: "db", Timestamp: timestamp, HexId: &spanHexId, TransactionId: &transactionId, TraceId: &traceId, Stacktrace: model.Stacktrace{&model.StacktraceFrame{Filename: "file.js", Lineno: 10}, &model.StacktraceFrame{Filename: "file2.js", Lineno: 11}}},
				&metric.Metric{Samples: []*metric.Sample{&metric.Sample{Name: "my-metric", Value: 99}}, Timestamp: timestamp},
				&errorm.Event{Exception: &errorm.Exception{Message: "hello world!", Stacktrace: model.Stacktrace{}}},
			},
		},
		{
			// optional timestamps
			body: strings.Join([]string{
				validMetadata(),
				`{"transaction": {"name": "tx1", "id": "1111222233334444", "trace_id": "abcdefabcdef01234567890123456789", "duration": 12, "type": "request"}}`,
				`{"span": {"name": "sp1","trace_id": "abcdefabcdef01234567890123456789", "duration": 20, "start": 10, "type": "db", "id": "0147258369abcdef", "transaction_id": "fedcba0123456789"}}`,
				`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: http.StatusAccepted,
			reported: []transform.Transformable{
				&transaction.Event{Name: &tx1, Id: "1111222233334444", Duration: 12, Type: "request", TraceId: &traceId},
				&span.Event{Name: "sp1", Duration: 20.0, Start: 10, Type: "db", HexId: &spanHexId, TransactionId: &transactionId, TraceId: &traceId},
				&metric.Metric{Timestamp: timestamp, Samples: []*metric.Sample{&metric.Sample{Name: "my-metric", Value: 99}}},
			},
		},
	} {
		transformables = []transform.Transformable{}
		bodyReader := bytes.NewBufferString(test.body)

		// set request time
		ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
		reader := decoder.NewNDJSONStreamReader(bodyReader)
		sp := StreamProcessor{}

		actualResponse := sp.HandleStream(ctx, map[string]interface{}{}, reader, report)

		assert.Equal(t, test.expectedCode, actualResponse.StatusCode(), "Failed at index %d: %s", idx, actualResponse.String())
		if test.err != nil {
			assert.Equal(t, test.err.String(), actualResponse.String(), "Failed at index %d", idx)
		} else {
			assert.Equal(t, reqTimestamp, reportedTCtx.RequestTime)
		}

		assert.Equal(t, test.reported, transformables)
	}
}

func TestV2HandlerReadError(t *testing.T) {
	var transformables []transform.Transformable
	report := func(ctx context.Context, p publish.PendingReq) error {
		transformables = append(transformables, p.Transformables...)
		return nil
	}

	body := strings.Join([]string{
		validMetadata(),
		`{"transaction": {"name": "tx1", "id": "8ace3f94cd01462c", "trace_id": "0123456789", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z"}}`,
		`{"span": {"name": "sp1", "duration": 20, "start": 10, "type": "db", "trace_id": "0123456789", "id": "0000111122223333", "timestamp": "2018-01-01T10:00:01Z", "transaction_id": "fedcba0123456789"}}`,
		`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
	}, "\n")

	bodyReader := bytes.NewBufferString(body)
	timeoutReader := iotest.TimeoutReader(bodyReader)

	reader := decoder.NewNDJSONStreamReader(timeoutReader)
	sp := StreamProcessor{}

	actualResponse := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, report)

	expected := &Result{
		Errors: map[StreamErrorType]errorDetails{
			"ERR_SERVER_ERROR": errorDetails{
				Count:   1,
				Message: "timeout",
			},
		},
	}

	assert.Equal(t, expected, actualResponse)
	assert.Equal(t, http.StatusInternalServerError, actualResponse.StatusCode(), actualResponse.String())
}

func TestV2HandlerReportingError(t *testing.T) {
	for _, test := range []struct {
		err          *Result
		expectedCode int
		report       func(ctx context.Context, p publish.PendingReq) error
	}{
		{
			err: &Result{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_SHUTTING_DOWN": errorDetails{
						Count:   1,
						Message: "server is shutting down",
					},
				},
				Accepted: 0,
				Dropped:  0,
				Invalid:  0,
			},
			expectedCode: 503,
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrChannelClosed
			},
		}, {
			err: &Result{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_QUEUE_FULL": errorDetails{
						Count:   2,
						Message: "queue is full",
					},
				},
				Accepted: 0,
				Dropped:  2,
				Invalid:  0,
			},
			expectedCode: 429,
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrFull
			},
		},
	} {
		body := strings.Join([]string{
			validMetadata(),
			`{"transaction": {"name": "tx1","trace_id": "01234567890123456789abcdefabcdef", "id": "8ace3f94462ab069", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z"}}`,
			`{"transaction": {"name": "tx1","trace_id": "01234567890123456789abcdefabcdef", "id": "8ace3f94462ab069", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z"}}`,
		}, "\n")

		bodyReader := bytes.NewBufferString(body)

		reader := decoder.NewNDJSONStreamReader(bodyReader)

		sp := StreamProcessor{}
		actualResponse := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, test.report)

		assert.Equal(t, test.expectedCode, actualResponse.StatusCode(), actualResponse.String())
		assert.Equal(t, test.err, actualResponse)
	}
}
