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

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
)

func TestHandlerReadStreamError(t *testing.T) {
	var accepted int
	processor := model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		events := batch.Transform(ctx)
		accepted += len(events)
		return nil
	})

	payload, err := ioutil.ReadFile("../../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	timeoutReader := iotest.TimeoutReader(bytes.NewReader(payload))

	sp := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})

	var actualResult Result
	err = sp.HandleStream(context.Background(), model.APMEvent{}, timeoutReader, 10, processor, &actualResult)
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
			context.Background(), model.APMEvent{},
			bytes.NewReader(payload), 10, processor, &actualResult,
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
		name: "TransactionsHugeTraces",
		path: "transactions-huge_traces.ndjson",
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
		name: "OpenTelemetryBridge",
		path: "otel-bridge.ndjson",
	}, {
		name: "SpanLinks",
		path: "span-links.ndjson",
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
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				Host:      model.Host{IP: []net.IP{net.ParseIP("192.0.0.1")}},
				Timestamp: reqTimestamp,
			}

			p := BackendProcessor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(context.Background(), baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
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
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Source:    model.Source{IP: net.ParseIP("192.0.0.1")},
				Client:    model.Client{IP: net.ParseIP("192.0.0.2")}, // X-Forwarded-For
				Timestamp: reqTimestamp,
			}

			p := RUMV2Processor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(context.Background(), baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
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
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Source:    model.Source{IP: net.ParseIP("192.0.0.1")},
				Client:    model.Client{IP: net.ParseIP("192.0.0.2")}, // X-Forwarded-For
				Timestamp: reqTimestamp,
			}

			p := RUMV3Processor(&config.Config{MaxEventSize: 100 * 1024})
			var actualResult Result
			err = p.HandleStream(context.Background(), baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
			require.NoError(t, err)
			assert.Equal(t, Result{Accepted: accepted}, actualResult)
		})
	}
}

func makeApproveEventsBatchProcessor(t *testing.T, name string, count *int) model.BatchProcessor {
	return model.ProcessBatchFunc(func(ctx context.Context, b *model.Batch) error {
		events := b.Transform(ctx)
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
