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
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

func TestHandlerReadStreamError(t *testing.T) {
	var accepted int
	processor := model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		events := batch.Transform(ctx, &transform.Config{})
		accepted += len(events)
		return nil
	})

	payload, err := ioutil.ReadFile("../../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	timeoutReader := iotest.TimeoutReader(bytes.NewReader(payload))

	sp := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})

	var actualResult Result
	err = sp.HandleStream(context.Background(), nil, &model.Metadata{}, timeoutReader, processor, &actualResult)
	assert.EqualError(t, err, "timeout")
	assert.Equal(t, Result{Accepted: accepted}, actualResult)
}

func TestHandlerReportingStreamError(t *testing.T) {
	payload, err := ioutil.ReadFile("../../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)

	for _, test := range []struct {
		name string
		err  error
	}{{
		name: "ShuttingDown",
		err:  publish.ErrChannelClosed,
	}, {
		name: "QueueFull",
		err:  publish.ErrFull,
	}} {
		sp := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
		processor := model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
			return test.err
		})

		var actualResult Result
		err := sp.HandleStream(
			context.Background(), nil, &model.Metadata{},
			bytes.NewReader(payload), processor, &actualResult,
		)
		assert.Equal(t, test.err, err)
		assert.Zero(t, actualResult)
	}
}

func TestIntegrationESOutput(t *testing.T) {
	for _, test := range []struct {
		name   string
		path   string
		errors []error // per-event errors
		err    error   // stream-level error
	}{{
		name: "Errors",
		path: "errors.ndjson",
	}, {
		name: "Transactions",
		path: "transactions.ndjson",
	}, {
		name: "Spans",
		path: "spans.ndjson",
	}, {
		name: "Metricsets",
		path: "metricsets.ndjson",
	}, {
		name: "Events",
		path: "events.ndjson",
	}, {
		name: "MinimalService",
		path: "minimal-service.ndjson",
	}, {
		name: "MetadataNullValues",
		path: "metadata-null-values.ndjson",
	}, {
		name: "OptionalTimestamps",
		path: "optional-timestamps.ndjson",
	}, {
		name: "InvalidEvent",
		path: "invalid-event.ndjson",
		errors: []error{
			&InvalidInputError{
				Message:  `decode error: data read error: v2.transactionRoot.Transaction: v2.transaction.ID: ReadString: expects " or n,`,
				Document: `{ "transaction": { "id": 12345, "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981, "span_count": { "started": 21 } } }   `,
			},
		},
	}, {
		name: "InvalidJSONEvent",
		path: "invalid-json-event.ndjson",
		errors: []error{
			&InvalidInputError{
				Message:  "invalid-json: did not recognize object type",
				Document: `{ "invalid-json" }`,
			},
		},
	}, {
		name: "InvalidJSONMetadata",
		path: "invalid-json-metadata.ndjson",
		err: &InvalidInputError{
			Message:  "decode error: data read error: v2.metadataRoot.Metadata: v2.metadata.readFieldHash: expect :,",
			Document: `{"metadata": {"invalid-json"}}`,
		},
	}, {
		name: "InvalidMetadata",
		path: "invalid-metadata.ndjson",
		err: &InvalidInputError{
			Message:  "validation error: 'metadata' required",
			Document: `{"metadata": {"user": null}}`,
		},
	}, {
		name: "InvalidMetadata2",
		path: "invalid-metadata-2.ndjson",
		err: &InvalidInputError{
			Message:  "validation error: 'metadata' required",
			Document: `{"not": "metadata"}`,
		},
	}, {
		name: "UnrecognizedEvent",
		path: "invalid-event-type.ndjson",
		errors: []error{
			&InvalidInputError{
				Message:  "tennis-court: did not recognize object type",
				Document: `{"tennis-court": {"name": "Centre Court, Wimbledon"}}`,
			},
		},
	}} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := ioutil.ReadFile(filepath.Join("../../testdata/intake-v2", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			reqDecoderMeta := &model.Metadata{System: model.System{IP: net.ParseIP("192.0.0.1")}}

			p := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(ctx, nil, reqDecoderMeta, bytes.NewReader(payload), batchProcessor, &actualResult)
			if test.err != nil {
				assert.Equal(t, test.err, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, Result{Accepted: accepted, Errors: test.errors}, actualResult)
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
			payload, err := ioutil.ReadFile(filepath.Join("../../testdata/intake-v2", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			reqDecoderMeta := model.Metadata{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Client:    model.Client{IP: net.ParseIP("192.0.0.1")}}

			p := RUMV2Processor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(ctx, nil, &reqDecoderMeta, bytes.NewReader(payload), batchProcessor, &actualResult)
			require.NoError(t, err)
			assert.Equal(t, Result{Accepted: accepted}, actualResult)
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
			payload, err := ioutil.ReadFile(filepath.Join("../../testdata/intake-v3", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntake%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			ctx := utility.ContextWithRequestTime(context.Background(), reqTimestamp)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			reqDecoderMeta := model.Metadata{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Client:    model.Client{IP: net.ParseIP("192.0.0.1")}}

			p := RUMV3Processor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(ctx, nil, &reqDecoderMeta, bytes.NewReader(payload), batchProcessor, &actualResult)
			require.NoError(t, err)
			assert.Equal(t, Result{Accepted: accepted}, actualResult)
		})
	}
}

func TestRUMAllowedServiceNames(t *testing.T) {
	payload, err := ioutil.ReadFile("../../testdata/intake-v2/transactions_spans_rum.ndjson")
	require.NoError(t, err)

	for _, test := range []struct {
		AllowServiceNames []string
		Allowed           bool
	}{{
		AllowServiceNames: nil,
		Allowed:           true, // none specified = all allowed
	}, {
		AllowServiceNames: []string{"apm-agent-js"}, // matches what's in test data
		Allowed:           true,
	}, {
		AllowServiceNames: []string{"reject_everything"},
		Allowed:           false,
	}} {
		p := RUMV2Processor(&config.Config{
			MaxEventSize: 100 * 1024,
			RumConfig:    config.RumConfig{AllowServiceNames: test.AllowServiceNames},
		})

		var result Result
		err := p.HandleStream(
			context.Background(), nil, &model.Metadata{}, bytes.NewReader(payload),
			modelprocessor.Nop{}, &result,
		)
		if test.Allowed {
			require.NoError(t, err)
			assert.Equal(t, Result{Accepted: 2}, result)
		} else {
			assert.EqualError(t, err, "service name is not allowed")
			assert.Equal(t, Result{Accepted: 0}, result)
		}
	}
}

func TestRateLimiting(t *testing.T) {
	payload, err := ioutil.ReadFile("../../testdata/intake-v2/ratelimit.ndjson")
	require.NoError(t, err)

	for _, test := range []struct {
		name     string
		lim      *rate.Limiter
		hit      int
		accepted int
		limited  bool
	}{{
		name:     "LimiterDenyAll",
		lim:      rate.NewLimiter(rate.Limit(0), 2),
		accepted: 0,
		limited:  true,
	}, {
		name:     "LimiterAllowAll",
		lim:      rate.NewLimiter(rate.Limit(40), 40*5),
		accepted: 19,
	}, {
		name:     "LimiterPartiallyUsedLimitAllow",
		lim:      rate.NewLimiter(rate.Limit(10), 10*2),
		hit:      10,
		accepted: 19,
	}, {
		name:     "LimiterPartiallyUsedLimitDeny",
		lim:      rate.NewLimiter(rate.Limit(7), 7*2),
		hit:      10,
		accepted: 10,
		limited:  true,
	}, {
		name:     "LimiterDeny",
		lim:      rate.NewLimiter(rate.Limit(6), 6*2),
		accepted: 10,
		limited:  true,
	}} {
		t.Run(test.name, func(t *testing.T) {
			if test.hit > 0 {
				assert.True(t, test.lim.AllowN(time.Now(), test.hit))
			}

			var actualResult Result
			err := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024}).HandleStream(
				context.Background(), test.lim, &model.Metadata{}, bytes.NewReader(payload), nopBatchProcessor{},
				&actualResult,
			)
			if test.limited {
				assert.Equal(t, ErrRateLimitExceeded, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, Result{Accepted: test.accepted}, actualResult)
		})
	}
}

func makeApproveEventsBatchProcessor(t *testing.T, name string, count *int) model.BatchProcessor {
	return model.ProcessBatchFunc(func(ctx context.Context, b *model.Batch) error {
		events := b.Transform(ctx, &transform.Config{DataStreams: true})
		*count += len(events)
		docs := beatertest.EncodeEventDocs(events...)
		approvaltest.ApproveEventDocs(t, name, docs)
		return nil
	})
}

type nopBatchProcessor struct{}

func (nopBatchProcessor) ProcessBatch(context.Context, *model.Batch) error {
	return nil
}
