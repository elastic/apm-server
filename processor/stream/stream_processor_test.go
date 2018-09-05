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
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"

	"github.com/elastic/apm-server/publish"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/transform"
)

func validMetadata() string {
	return `{"metadata": {"service": {"name": "myservice", "agent": {"name": "test", "version": "1.0"}}}}`
}

func assertApproveResult(t *testing.T, actualResponse *Result, name string) {
	resultName := fmt.Sprintf("approved-stream-result/testIntegrationResult%s", name)
	resultJSON, err := json.Marshal(actualResponse)
	require.NoError(t, err)
	tests.AssertApproveResult(t, resultName, resultJSON)
}

func TestV2HandlerReadStreamError(t *testing.T) {
	var transformables []transform.Transformable
	report := func(ctx context.Context, p publish.PendingReq) error {
		transformables = append(transformables, p.Transformables...)
		return nil
	}
	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	bodyReader := bytes.NewBuffer(b)
	timeoutReader := iotest.TimeoutReader(bodyReader)

	reader := decoder.NewNDJSONStreamReader(timeoutReader, 100*1024)
	sp := StreamProcessor{}

	actualResult := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, report)
	assertApproveResult(t, actualResult, "ReadError")
}

func TestV2HandlerReportingStreamError(t *testing.T) {
	for _, test := range []struct {
		name   string
		report func(ctx context.Context, p publish.PendingReq) error
	}{
		{
			name: "ShuttingDown",
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrChannelClosed
			},
		}, {
			name: "QueueFull",
			report: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrFull
			},
		},
	} {

		b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
		require.NoError(t, err)
		bodyReader := bytes.NewBuffer(b)

		reader := decoder.NewNDJSONStreamReader(bodyReader, 1024)

		sp := StreamProcessor{}
		actualResult := sp.HandleStream(context.Background(), map[string]interface{}{}, reader, test.report)
		assertApproveResult(t, actualResult, test.name)
	}
}

func TestIntegration(t *testing.T) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		var events []beat.Event
		for _, transformable := range p.Transformables {
			events = append(events, transformable.Transform(p.Tcontext)...)
		}
		name := ctx.Value("name").(string)
		verifyErr := tests.ApproveEvents(events, name, nil)
		if verifyErr != nil {
			assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
		}
		return nil
	}

	for _, test := range []struct {
		path string
		name string
	}{
		{path: "errors.ndjson", name: "Errors"},
		{path: "transactions.ndjson", name: "Transactions"},
		{path: "spans.ndjson", name: "Spans"},
		{path: "metrics.ndjson", name: "Metrics"},
		{path: "events.ndjson", name: "Events"},
		{path: "minimal-service.ndjson", name: "MinimalService"},
		{path: "metadata-null-values.ndjson", name: "MetadataNullValues"},
		{path: "invalid-event.ndjson", name: "InvalidEvent"},
		{path: "invalid-json-event.ndjson", name: "InvalidJSONEvent"},
		{path: "invalid-json-metadata.ndjson", name: "InvalidJSONMetadata"},
		{path: "invalid-metadata.ndjson", name: "InvalidMetadata"},
		{path: "invalid-metadata-2.ndjson", name: "InvalidMetadata2"},
		{path: "unrecognized-event.ndjson", name: "UnrecognizedEvent"},
		{path: "optional-timestamps.ndjson", name: "OptionalTimestamps"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v2/", test.path))
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			name := fmt.Sprintf("approved-es-documents/testV2IntakeIntegration%s", test.name)
			ctx := context.WithValue(context.Background(), "name", name)
			reqTimestamp, err := time.Parse(time.RFC3339, "2018-08-01T10:00:00Z")
			ctx = utility.ContextWithRequestTime(ctx, reqTimestamp)

			reader := decoder.NewNDJSONStreamReader(bodyReader, 100*1024)

			reqDecoderMeta := map[string]interface{}{
				"system": map[string]interface{}{
					"ip": "192.0.0.1",
				},
			}

			actualResult := (&StreamProcessor{}).HandleStream(ctx, reqDecoderMeta, reader, report)
			assertApproveResult(t, actualResult, test.name)
		})
	}
}

func TestRateLimiting(t *testing.T) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		for range p.Transformables {
		}
		return nil
	}

	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/ratelimit.ndjson")
	require.NoError(t, err)
	r := bytes.NewReader(b)

	// no rate limiter passed - all events processed
	reader := decoder.NewNDJSONStreamReader(r, math.MaxInt32)
	ctx := context.Background()
	actualResult := (&StreamProcessor{}).HandleStream(ctx, map[string]interface{}{}, reader, report)
	assertApproveResult(t, actualResult, "Unlimited")

	// rate limiter passed
	for _, test := range []struct {
		limit   int
		minTime time.Duration
		hit     int
		name    string
	}{
		{limit: 0, name: "DenyAll"},
		{limit: 40, name: "AllowAll"},
		{limit: 10, hit: 10, minTime: time.Second, name: "SlowedDownBatch"},
		{limit: 7, minTime: 600 * time.Millisecond, name: "AllowWithWait"},
		{limit: 6, name: "Forbidden"},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Now()
			r.Reset(b)
			reader = decoder.NewNDJSONStreamReader(r, math.MaxInt32)

			limiter := rate.NewLimiter(rate.Limit(test.limit), test.limit*2)
			require.True(t, limiter.AllowN(time.Now(), test.hit))
			ctx = context.WithValue(context.Background(), RateLimiterKey, limiter)
			actualResult := (&StreamProcessor{}).HandleStream(ctx, map[string]interface{}{}, reader, report)
			execTime := time.Now().Sub(start)
			assert.True(t, execTime > test.minTime,
				fmt.Sprintf("%v (ExecTime), %v (expected Min)", execTime, test.minTime))
			assertApproveResult(t, actualResult, test.name)
		})
	}
}
