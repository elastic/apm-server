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
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/approvaltest"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/publish"
)

func TestHandlerReadStreamError(t *testing.T) {
	var accepted int
	processor := model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		events := batch.Transform(ctx)
		accepted += len(events)
		return nil
	})

	payload, err := os.ReadFile("../../../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	timeoutReader := iotest.TimeoutReader(bytes.NewReader(payload))

	sp := BackendProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    make(chan struct{}, 1),
	})

	var actualResult Result
	err = sp.HandleStream(context.Background(), false, model.APMEvent{}, timeoutReader, 10, processor, &actualResult)
	assert.EqualError(t, err, "timeout")
	assert.Equal(t, Result{Accepted: accepted}, actualResult)
}

func TestHandlerReportingStreamError(t *testing.T) {
	payload, err := os.ReadFile("../../../testdata/intake-v2/transactions.ndjson")
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
		sp := BackendProcessor(Config{
			MaxEventSize: 100 * 1024,
			Semaphore:    make(chan struct{}, 1),
		})
		processor := model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
			return test.err
		})

		var actualResult Result
		err := sp.HandleStream(
			context.Background(), false, model.APMEvent{},
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
		name: "Logs",
		path: "logs.ndjson",
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
			payload, err := os.ReadFile(filepath.Join("../../../testdata/intake-v2", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				Host:      model.Host{IP: []netip.Addr{netip.MustParseAddr("192.0.0.1")}},
				Timestamp: reqTimestamp,
			}

			p := BackendProcessor(Config{
				MaxEventSize: 100 * 1024,
				Semaphore:    make(chan struct{}, 1),
			})
			var actualResult Result
			err = p.HandleStream(context.Background(), false, baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
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
			payload, err := os.ReadFile(filepath.Join("../../../testdata/intake-v2", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntakeIntegration%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Source:    model.Source{IP: netip.MustParseAddr("192.0.0.1")},
				Client:    model.Client{IP: netip.MustParseAddr("192.0.0.2")}, // X-Forwarded-For
				Timestamp: reqTimestamp,
			}

			p := RUMV2Processor(Config{
				MaxEventSize: 100 * 1024,
				Semaphore:    make(chan struct{}, 1),
			})
			var actualResult Result
			err = p.HandleStream(context.Background(), false, baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
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
			payload, err := os.ReadFile(filepath.Join("../../../testdata/intake-v3", test.path))
			require.NoError(t, err)

			var accepted int
			name := fmt.Sprintf("test_approved_es_documents/testIntake%s", test.name)
			reqTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)
			batchProcessor := makeApproveEventsBatchProcessor(t, name, &accepted)

			baseEvent := model.APMEvent{
				UserAgent: model.UserAgent{Original: "rum-2.0"},
				Source:    model.Source{IP: netip.MustParseAddr("192.0.0.1")},
				Client:    model.Client{IP: netip.MustParseAddr("192.0.0.2")}, // X-Forwarded-For
				Timestamp: reqTimestamp,
			}

			p := RUMV3Processor(Config{
				MaxEventSize: 100 * 1024,
				Semaphore:    make(chan struct{}, 1),
			})
			var actualResult Result
			err = p.HandleStream(context.Background(), false, baseEvent, bytes.NewReader(payload), 10, batchProcessor, &actualResult)
			require.NoError(t, err)
			assert.Equal(t, Result{Accepted: accepted}, actualResult)
		})
	}
}

func TestLabelLeak(t *testing.T) {
	payload := `{"metadata": {"service": {"name": "testsvc", "environment": "staging", "version": null, "agent": {"name": "python", "version": "6.9.1"}, "language": {"name": "python", "version": "3.10.4"}, "runtime": {"name": "CPython", "version": "3.10.4"}, "framework": {"name": "flask", "version": "2.1.1"}}, "process": {"pid": 2112739, "ppid": 2112738, "argv": ["/home/stuart/workspace/sdh/581/venv/lib/python3.10/site-packages/flask/__main__.py", "run"], "title": null}, "system": {"hostname": "slaptop", "architecture": "x86_64", "platform": "linux"}, "labels": {"ci_commit": "unknown", "numeric": 1}}}
{"transaction": {"id": "88dee29a6571b948", "trace_id": "ba7f5d18ac4c7f39d1ff070c79b2bea5", "name": "GET /withlabels", "type": "request", "duration": 1.6199999999999999, "result": "HTTP 2xx", "timestamp": 1652185276804681, "outcome": "success", "sampled": true, "span_count": {"started": 0, "dropped": 0}, "sample_rate": 1.0, "context": {"request": {"env": {"REMOTE_ADDR": "127.0.0.1", "SERVER_NAME": "127.0.0.1", "SERVER_PORT": "5000"}, "method": "GET", "socket": {"remote_address": "127.0.0.1"}, "cookies": {}, "headers": {"host": "localhost:5000", "user-agent": "curl/7.81.0", "accept": "*/*", "app-os": "Android", "content-type": "application/json; charset=utf-8", "content-length": "29"}, "url": {"full": "http://localhost:5000/withlabels?second_with_labels", "protocol": "http:", "hostname": "localhost", "pathname": "/withlabels", "port": "5000", "search": "?second_with_labels"}}, "response": {"status_code": 200, "headers": {"Content-Type": "application/json", "Content-Length": "14"}}, "tags": {"appOs": "Android", "email_set": "hello@hello.com", "time_set": 1652185276}}}}
{"transaction": {"id": "ba5c6d6c1ab44bd1", "trace_id": "88c0a00431531a80c5ca9a41fe115f41", "name": "GET /nolabels", "type": "request", "duration": 0.652, "result": "HTTP 2xx", "timestamp": 1652185278813952, "outcome": "success", "sampled": true, "span_count": {"started": 0, "dropped": 0}, "sample_rate": 1.0, "context": {"request": {"env": {"REMOTE_ADDR": "127.0.0.1", "SERVER_NAME": "127.0.0.1", "SERVER_PORT": "5000"}, "method": "GET", "socket": {"remote_address": "127.0.0.1"}, "cookies": {}, "headers": {"host": "localhost:5000", "user-agent": "curl/7.81.0", "accept": "*/*"}, "url": {"full": "http://localhost:5000/nolabels?third_no_label", "protocol": "http:", "hostname": "localhost", "pathname": "/nolabels", "port": "5000", "search": "?third_no_label"}}, "response": {"status_code": 200, "headers": {"Content-Type": "text/html; charset=utf-8", "Content-Length": "14"}}, "tags": {}}}}`

	baseEvent := model.APMEvent{
		Host: model.Host{IP: []netip.Addr{netip.MustParseAddr("192.0.0.1")}},
	}

	var processed *model.Batch
	batchProcessor := model.ProcessBatchFunc(func(_ context.Context, b *model.Batch) error {
		processed = b
		return nil
	})

	p := BackendProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    make(chan struct{}, 1),
	})
	var actualResult Result
	err := p.HandleStream(context.Background(), false, baseEvent, strings.NewReader(payload), 10, batchProcessor, &actualResult)
	require.NoError(t, err)

	txs := *processed
	assert.Len(t, txs, 2)
	// Assert first tx
	assert.Equal(t, model.NumericLabels{
		"time_set": {Value: 1652185276},
		"numeric":  {Global: true, Value: 1},
	}, txs[0].NumericLabels)
	assert.Equal(t, model.Labels{
		"appOs":     {Value: "Android"},
		"email_set": {Value: "hello@hello.com"},
		"ci_commit": {Global: true, Value: "unknown"},
	}, txs[0].Labels)

	// Assert second tx
	assert.Equal(t, model.NumericLabels{"numeric": {Global: true, Value: 1}}, txs[1].NumericLabels)
	assert.Equal(t, model.Labels{"ci_commit": {Global: true, Value: "unknown"}}, txs[1].Labels)
}

func makeApproveEventsBatchProcessor(t *testing.T, name string, count *int) model.BatchProcessor {
	return model.ProcessBatchFunc(func(ctx context.Context, b *model.Batch) error {
		var docs [][]byte
		for _, event := range *b {
			data, err := event.MarshalJSON()
			require.NoError(t, err)
			docs = append(docs, data)
		}
		*count += len(docs)
		approvaltest.ApproveEventDocs(t, name, docs)
		return nil
	})
}

func TestConcurrentAsync(t *testing.T) {
	smallBatch := []byte(`{"metadata": {"service": {"name": "testsvc", "environment": "staging", "version": null, "agent": {"name": "python", "version": "6.9.1"}, "language": {"name": "python", "version": "3.10.4"}, "runtime": {"name": "CPython", "version": "3.10.4"}, "framework": {"name": "flask", "version": "2.1.1"}}, "process": {"pid": 2112739, "ppid": 2112738, "argv": ["/home/stuart/workspace/sdh/581/venv/lib/python3.10/site-packages/flask/__main__.py", "run"], "title": null}, "system": {"hostname": "slaptop", "architecture": "x86_64", "platform": "linux"}, "labels": {"ci_commit": "unknown", "numeric": 1}}}
{"transaction": {"id": "88dee29a6571b948", "trace_id": "ba7f5d18ac4c7f39d1ff070c79b2bea5", "name": "GET /withlabels", "type": "request", "duration": 1.6199999999999999, "result": "HTTP 2xx", "timestamp": 1652185276804681, "outcome": "success", "sampled": true, "span_count": {"started": 0, "dropped": 0}, "sample_rate": 1.0, "context": {"request": {"env": {"REMOTE_ADDR": "127.0.0.1", "SERVER_NAME": "127.0.0.1", "SERVER_PORT": "5000"}, "method": "GET", "socket": {"remote_address": "127.0.0.1"}, "cookies": {}, "headers": {"host": "localhost:5000", "user-agent": "curl/7.81.0", "accept": "*/*", "app-os": "Android", "content-type": "application/json; charset=utf-8", "content-length": "29"}, "url": {"full": "http://localhost:5000/withlabels?second_with_labels", "protocol": "http:", "hostname": "localhost", "pathname": "/withlabels", "port": "5000", "search": "?second_with_labels"}}, "response": {"status_code": 200, "headers": {"Content-Type": "application/json", "Content-Length": "14"}}, "tags": {"appOs": "Android", "email_set": "hello@hello.com", "time_set": 1652185276}}}}`)
	bigBatch, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "intake-v2", "heavy.ndjson"))
	assert.NoError(t, err)

	base := model.APMEvent{Host: model.Host{IP: []netip.Addr{netip.MustParseAddr("192.0.0.1")}}}
	type testCase struct {
		payload       []byte
		sem, requests int
		fullSem       bool
	}

	test := func(tc testCase) (pResult Result) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		p := BackendProcessor(Config{
			MaxEventSize: 100 * 1024,
			Semaphore:    make(chan struct{}, tc.sem),
		})
		if tc.fullSem {
			for i := 0; i < tc.sem; i++ {
				p.sem <- struct{}{}
			}
		}
		handleStream := func(ctx context.Context, bp *accountProcessor) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var result Result
				err := p.HandleStream(ctx, true, base, bytes.NewReader(tc.payload), 10, bp, &result)
				if err != nil {
					result.LimitedAdd(err)
				}
				if !tc.fullSem {
					select {
					case <-bp.batch:
					case <-ctx.Done():
					}
				}
				mu.Lock()
				if len(result.Errors) > 0 {
					pResult.Errors = append(pResult.Errors, result.Errors...)
				}
				mu.Unlock()
			}()
		}
		batchProcessor := &accountProcessor{batch: make(chan *model.Batch, tc.requests)}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		for i := 0; i < tc.requests; i++ {
			handleStream(ctx, batchProcessor)
		}
		wg.Wait()
		if !tc.fullSem {
			// Try to acquire the lock to make sure all the requests have been handled
			// and the locks have been released.
			for i := 0; i < tc.sem; i++ {
				p.semAcquire(context.Background(), false)
			}
		}
		processed := atomic.LoadUint64(&batchProcessor.processed)
		pResult.Accepted += int(processed)
		return
	}

	t.Run("semaphore_full", func(t *testing.T) {
		res := test(testCase{
			sem:      2,
			requests: 3,
			fullSem:  true,
			payload:  smallBatch,
		})
		assert.Equal(t, 0, res.Accepted)
		assert.Equal(t, 3, len(res.Errors))
		for _, err := range res.Errors {
			assert.ErrorIs(t, err, publish.ErrFull)
		}
	})
	t.Run("semaphore_undersized", func(t *testing.T) {
		res := test(testCase{
			sem:      2,
			requests: 100,
			payload:  bigBatch,
		})
		// When the semaphore is full, `publish.ErrFull` is returned.
		assert.Greater(t, len(res.Errors), 0)
		for _, err := range res.Errors {
			assert.EqualError(t, err, publish.ErrFull.Error())
		}
	})
	t.Run("semaphore_empty", func(t *testing.T) {
		res := test(testCase{
			sem:      5,
			requests: 5,
			payload:  smallBatch,
		})
		assert.Equal(t, 5, res.Accepted)
		assert.Equal(t, 0, len(res.Errors))

		res = test(testCase{
			sem:      5,
			requests: 5,
			payload:  bigBatch,
		})
		assert.GreaterOrEqual(t, res.Accepted, 5)
		// all the request will return with an error since only 50 events of
		// each (5 requests * batch size) will be processed.
		assert.Equal(t, 5, len(res.Errors))
	})
	t.Run("semaphore_empty_incorrect_metadata", func(t *testing.T) {
		res := test(testCase{
			sem:      5,
			requests: 5,
			payload:  []byte(`{"metadata": {"siervice":{}}}`),
		})
		assert.Equal(t, 0, res.Accepted)
		assert.Len(t, res.Errors, 5)

		incorrectEvent := []byte(`{"metadata": {"service": {"name": "testsvc", "environment": "staging", "version": null, "agent": {"name": "python", "version": "6.9.1"}, "language": {"name": "python", "version": "3.10.4"}, "runtime": {"name": "CPython", "version": "3.10.4"}, "framework": {"name": "flask", "version": "2.1.1"}}, "process": {"pid": 2112739, "ppid": 2112738, "argv": ["/home/stuart/workspace/sdh/581/venv/lib/python3.10/site-packages/flask/__main__.py", "run"], "title": null}, "system": {"hostname": "slaptop", "architecture": "x86_64", "platform": "linux"}, "labels": {"ci_commit": "unknown", "numeric": 1}}}
{"some_incorrect_event": {}}`)
		res = test(testCase{
			sem:      5,
			requests: 2,
			payload:  incorrectEvent,
		})
		assert.Equal(t, 0, res.Accepted)
		assert.Len(t, res.Errors, 2)
	})
}

type nopBatchProcessor struct{}

func (nopBatchProcessor) ProcessBatch(context.Context, *model.Batch) error {
	return nil
}

type accountProcessor struct {
	batch     chan *model.Batch
	processed uint64
}

func (p *accountProcessor) ProcessBatch(ctx context.Context, b *model.Batch) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if p.batch != nil {
		select {
		case p.batch <- b:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	atomic.AddUint64(&p.processed, 1)
	return nil
}
