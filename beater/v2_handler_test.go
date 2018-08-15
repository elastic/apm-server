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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/transaction"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/transform"
)

func validMetadata() string {
	return `{"metadata": {"service": {"name": "myservice", "agent": {"name": "test", "version": "1.0"}}}}`
}

func TestV2Handler(t *testing.T) {
	var transformables []transform.Transformable
	var reportedTCtx *transform.Context
	report := func(ctx context.Context, p pendingReq) error {
		transformables = append(transformables, p.transformables...)
		reportedTCtx = p.tcontext
		return nil
	}

	c := defaultConfig("7.0.0")

	handler := (&v2Route{backendRouteType}).Handler(c, report)

	tx1 := "tx1"
	timestamp, err := time.Parse(time.RFC3339, "2018-01-01T10:00:00Z")
	assert.NoError(t, err)
	reqTimestamp, err := time.Parse(time.RFC3339, "2018-01-02T10:00:00Z")
	assert.NoError(t, err)

	for idx, test := range []struct {
		body         string
		contentType  string
		err          *StreamResponse
		expectedCode int
		reported     []transform.Transformable
	}{
		{
			body:        "",
			contentType: "",
			err: &StreamResponse{
				Errors: map[StreamErrorType]errorDetails{
					"ERR_CONTENT_TYPE": errorDetails{
						Count:   1,
						Message: "invalid content-type. Expected 'application/x-ndjson'",
					},
				},
				Accepted: -1,
				Dropped:  -1,
				Invalid:  1,
			},
			expectedCode: 400,
			reported:     []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				`{"metadata": {}}`,
				`{"span": {}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: 400,
			err: &StreamResponse{
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
				Dropped: 1,
			},
			reported: []transform.Transformable{},
		},
		{
			body: strings.Join([]string{
				validMetadata(),
				`{"transaction": {"name": "tx1", "id": "8ace3f94-cd01-462c-b069-57dc28ebdfc8", "duration": 12, "type": "request", "timestamp": "2018-01-01T10:00:00Z"}}`,
				`{"span": {"name": "sp1", "duration": 20, "start": 10, "type": "db", "timestamp": "2018-01-01T10:00:00Z"}}`,
				`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: http.StatusAccepted,
			reported: []transform.Transformable{
				&transaction.Event{Name: &tx1, Id: "8ace3f94-cd01-462c-b069-57dc28ebdfc8", Duration: 12, Type: "request", Spans: []*span.Span{}, Timestamp: timestamp},
				&span.Span{Name: "sp1", Duration: 20.0, Start: 10, Type: "db", Timestamp: timestamp},
				&metric.Metric{Samples: []*metric.Sample{&metric.Sample{Name: "my-metric", Value: 99}}, Timestamp: timestamp},
			},
		},
		{
			// optional timestamps
			body: strings.Join([]string{
				validMetadata(),
				`{"transaction": {"name": "tx1", "id": "8ace3f94-cd01-462c-b069-57dc28ebdfc8", "duration": 12, "type": "request"}}`,
				`{"span": {"name": "sp1", "duration": 20, "start": 10, "type": "db"}}`,
				`{"metric": {"samples": {"my-metric": {"value": 99}}, "timestamp": "2018-01-01T10:00:00Z"}}`,
			}, "\n"),
			contentType:  "application/x-ndjson",
			expectedCode: http.StatusAccepted,
			reported: []transform.Transformable{
				&transaction.Event{Name: &tx1, Id: "8ace3f94-cd01-462c-b069-57dc28ebdfc8", Duration: 12, Type: "request", Spans: []*span.Span{}},
				&span.Span{Name: "sp1", Duration: 20.0, Start: 10, Type: "db"},
				&metric.Metric{Timestamp: timestamp, Samples: []*metric.Sample{&metric.Sample{Name: "my-metric", Value: 99}}},
			},
		},
	} {
		transformables = []transform.Transformable{}
		bodyReader := bytes.NewBufferString(test.body)

		r := httptest.NewRequest("POST", "/v2/intake", bodyReader)
		r.Header.Add("Content-Type", test.contentType)

		w := httptest.NewRecorder()

		// set request time
		r = r.WithContext(context.WithValue(r.Context(), requestTimeContextKey, reqTimestamp))

		handler.ServeHTTP(w, r)

		assert.Equal(t, test.expectedCode, w.Code, "Failed at index %d: %s", idx, w.Body.String())
		if test.err != nil {
			var actualResponse StreamResponse
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &actualResponse))
			assert.Equal(t, *test.err, actualResponse, "Failed at index %d", idx)
		} else {
			assert.Equal(t, reqTimestamp, reportedTCtx.RequestTime)
		}

		assert.Equal(t, test.reported, transformables)
	}

}
