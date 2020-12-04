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
	"net"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

func assertApproveResult(t *testing.T, actualResponse *Result, name string) {
	resultName := fmt.Sprintf("test_approved_stream_result/testIntegrationResult%s", name)
	resultJSON, err := json.Marshal(actualResponse)
	require.NoError(t, err)
	approvaltest.ApproveJSON(t, resultName, resultJSON)
}

func TestHandlerReadStreamError(t *testing.T) {
	var pendingReqs []publish.PendingReq
	report := tests.TestReporter(&pendingReqs)

	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	bodyReader := bytes.NewBuffer(b)
	timeoutReader := iotest.TimeoutReader(bodyReader)

	sp := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
	actualResult := sp.HandleStream(context.Background(), nil, &model.Metadata{}, timeoutReader, report)
	assertApproveResult(t, actualResult, "ReadError")
}

func TestHandlerReportingStreamError(t *testing.T) {
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

		sp := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
		actualResult := sp.HandleStream(context.Background(), nil, &model.Metadata{}, bodyReader, test.report)
		assertApproveResult(t, actualResult, test.name)
	}
}

func TestIntegrationESOutput(t *testing.T) {
	for _, test := range []struct {
		path string
		name string
	}{
		{path: "errors.ndjson", name: "Errors"},
		{path: "transactions.ndjson", name: "Transactions"},
		{path: "spans.ndjson", name: "Spans"},
		{path: "metricsets.ndjson", name: "Metricsets"},
		{path: "events.ndjson", name: "Events"},
		{path: "minimal-service.ndjson", name: "MinimalService"},
		{path: "metadata-null-values.ndjson", name: "MetadataNullValues"},
		{path: "invalid-event.ndjson", name: "InvalidEvent"},
		{path: "invalid-json-event.ndjson", name: "InvalidJSONEvent"},
		{path: "invalid-json-metadata.ndjson", name: "InvalidJSONMetadata"},
		{path: "invalid-metadata.ndjson", name: "InvalidMetadata"},
		{path: "invalid-metadata-2.ndjson", name: "InvalidMetadata2"},
		{path: "invalid-event-type.ndjson", name: "UnrecognizedEvent"},
		{path: "optional-timestamps.ndjson", name: "OptionalTimestamps"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v2/", test.path))
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
			report := makeApproveEventsReporter(t, name)

			reqDecoderMeta := &model.Metadata{System: model.System{IP: net.ParseIP("192.0.0.1")}}

			p := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
			actualResult := p.HandleStream(ctx, nil, reqDecoderMeta, bodyReader, report)
			assertApproveResult(t, actualResult, test.name)
		})
	}
}

func TestIntegrationRum(t *testing.T) {
	for _, test := range []struct {
		path string
		name string
	}{
		{path: "errors_rum.ndjson", name: "RumErrors"},
		{path: "transactions_spans_rum.ndjson", name: "RumTransactions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v2/", test.path))
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			ctx := context.WithValue(context.Background(), "name", name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx = utility.ContextWithRequestTime(ctx, reqTimestamp)
			report := makeApproveEventsReporter(t, name)

			reqDecoderMeta := model.Metadata{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Client:    model.Client{IP: net.ParseIP("192.0.0.1")}}

			p := RUMV2Processor(&config.Config{MaxEventSize: 100 * 1024})
			actualResult := p.HandleStream(ctx, nil, &reqDecoderMeta, bodyReader, report)
			assertApproveResult(t, actualResult, test.name)
		})
	}
}

func TestRUMV3(t *testing.T) {
	for _, test := range []struct {
		path string
		name string
	}{
		{path: "rum_errors.ndjson", name: "RUMV3Errors"},
		{path: "rum_events.ndjson", name: "RUMV3Events"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v3/", test.path))
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			name := fmt.Sprintf("test_approved_es_documents/testIntake%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
			report := makeApproveEventsReporter(t, name)

			reqDecoderMeta := model.Metadata{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Client:    model.Client{IP: net.ParseIP("192.0.0.1")}}

			p := RUMV3Processor(&config.Config{MaxEventSize: 100 * 1024})
			actualResult := p.HandleStream(ctx, nil, &reqDecoderMeta, bodyReader, report)
			assertApproveResult(t, actualResult, test.name)
		})
	}
}

func TestRateLimiting(t *testing.T) {
	report := func(ctx context.Context, p publish.PendingReq) error {
		return nil
	}

	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/ratelimit.ndjson")
	require.NoError(t, err)
	for _, test := range []struct {
		name string
		lim  *rate.Limiter
		hit  int
	}{
		{name: "NoLimiter"},
		{name: "LimiterDenyAll", lim: rate.NewLimiter(rate.Limit(0), 2)},
		{name: "LimiterAllowAll", lim: rate.NewLimiter(rate.Limit(40), 40*5)},
		{name: "LimiterPartiallyUsedLimitAllow", lim: rate.NewLimiter(rate.Limit(10), 10*2), hit: 10},
		{name: "LimiterPartiallyUsedLimitDeny", lim: rate.NewLimiter(rate.Limit(7), 7*2), hit: 10},
		{name: "LimiterDeny", lim: rate.NewLimiter(rate.Limit(6), 6*2)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.hit > 0 {
				assert.True(t, test.lim.AllowN(time.Now(), test.hit))
			}

			actualResult := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024}).HandleStream(
				context.Background(), test.lim, &model.Metadata{}, bytes.NewReader(b), report)
			assertApproveResult(t, actualResult, test.name)
		})
	}
}

func makeApproveEventsReporter(t *testing.T, name string) publish.Reporter {
	return func(ctx context.Context, p publish.PendingReq) error {
		var events []beat.Event
		for _, transformable := range p.Transformables {
			events = append(events, transformable.Transform(ctx, &transform.Config{DataStreams: true})...)
		}
		docs := beatertest.EncodeEventDocs(events...)
		approvaltest.ApproveEventDocs(t, name, docs)
		return nil
	}
}
