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

package elasticapm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestHandleStreamReaderError(t *testing.T) {
	readErr := errors.New("read failed")
	var calls int
	var reader readerFunc = func(p []byte) (int, error) {
		calls++
		if calls > 1 {
			return 0, readErr
		}
		buf := bytes.NewBuffer(nil)
		buf.WriteString(validMetadata + "\n")
		for i := 0; i < 5; i++ {
			buf.WriteString(validTransaction + "\n")
		}
		return copy(p, buf.Bytes()), nil
	}

	sp := NewProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    semaphore.NewWeighted(1),
	})

	var actualResult Result
	err := sp.HandleStream(
		context.Background(), &modelpb.APMEvent{},
		reader, 10, nopBatchProcessor{}, &actualResult,
	)
	assert.ErrorIs(t, err, readErr)
	assert.Equal(t, Result{Accepted: 5}, actualResult)
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

func TestHandleStreamBatchProcessorError(t *testing.T) {
	payload := validMetadata + "\n" + validTransaction + "\n"
	for _, test := range []struct {
		name string
		err  error
	}{{
		name: "NotQueueFull",
		err:  errors.New("queue is not full, something else is wrong"),
	}} {
		sp := NewProcessor(Config{
			MaxEventSize: 100 * 1024,
			Semaphore:    semaphore.NewWeighted(1),
		})
		processor := modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
			return test.err
		})

		var actualResult Result
		err := sp.HandleStream(
			context.Background(), &modelpb.APMEvent{},
			strings.NewReader(payload), 10, processor, &actualResult,
		)
		assert.ErrorIs(t, err, test.err)
		assert.Zero(t, actualResult)
	}
}

func TestHandleStreamErrors(t *testing.T) {
	var (
		invalidEvent        = `{ "transaction": { "id": 12345, "trace_id": "0123456789abcdef0123456789abcdef", "parent_id": "abcdefabcdef01234567", "type": "request", "duration": 32.592981, "span_count": { "started": 21 } } }   `
		invalidJSONEvent    = `{ "invalid-json" }`
		invalidJSONMetadata = `{"metadata": {"invalid-json"}}`
		invalidMetadata     = `{"metadata": {"user": null}}`
		invalidMetadata2    = `{"not": "metadata"}`
		invalidEventType    = `{"tennis-court": {"name": "Centre Court, Wimbledon"}}`
		tooLargeEvent       = strings.Repeat("*", len(validMetadata)*2)
	)

	for _, test := range []struct {
		name     string
		payload  string
		tooLarge int
		invalid  int
		errors   []error // per-event errors
		err      error   // stream-level error
	}{{
		name:    "InvalidEvent",
		payload: validMetadata + "\n" + invalidEvent + "\n",
		invalid: 1,
		errors: []error{
			&InvalidInputError{
				Message:  `decode error: data read error: v2.transactionRoot.Transaction: v2.transaction.ID: ReadString: expects " or n,`,
				Document: invalidEvent,
			},
		},
	}, {
		name:    "InvalidJSONEvent",
		payload: validMetadata + "\n" + invalidJSONEvent + "\n",
		invalid: 1,
		errors: []error{
			&InvalidInputError{
				Message:  `did not recognize object type: "invalid-json"`,
				Document: invalidJSONEvent,
			},
		},
	}, {
		name:    "InvalidJSONMetadata",
		payload: invalidJSONMetadata + "\n",
		err: &InvalidInputError{
			Message:  "decode error: data read error: v2.metadataRoot.Metadata: v2.metadata.readFieldHash: expect :,",
			Document: invalidJSONMetadata,
		},
	}, {
		name:    "InvalidMetadata",
		payload: invalidMetadata + "\n",
		err: &InvalidInputError{
			Message:  "validation error: 'metadata' required",
			Document: invalidMetadata,
		},
	}, {
		name:    "InvalidMetadata2",
		payload: invalidMetadata2 + "\n",
		err: fmt.Errorf("cannot read metadata in stream: %w", &InvalidInputError{
			Message:  `"metadata" or "m" required`,
			Document: invalidMetadata2,
		}),
	}, {
		name:    "UnrecognizedEvent",
		payload: validMetadata + "\n" + invalidEventType + "\n",
		invalid: 1,
		errors: []error{
			&InvalidInputError{
				Message:  `did not recognize object type: "tennis-court"`,
				Document: invalidEventType,
			},
		},
	}, {
		name: "EmptyEvent",
	}, {
		name:     "TooLargeEvent",
		payload:  validMetadata + "\n" + tooLargeEvent + "\n",
		tooLarge: 1,
		errors: []error{
			&InvalidInputError{
				TooLarge: true,
				Message:  "event exceeded the permitted size",
				Document: tooLargeEvent[:len(validMetadata)+1],
			},
		},
	}} {
		t.Run(test.name, func(t *testing.T) {
			var actualResult Result
			p := NewProcessor(Config{
				MaxEventSize: len(validMetadata) + 1,
				Semaphore:    semaphore.NewWeighted(1),
			})
			err := p.HandleStream(
				context.Background(), &modelpb.APMEvent{},
				strings.NewReader(test.payload), 10,
				nopBatchProcessor{}, &actualResult,
			)
			assert.Equal(t, test.err, err)
			assert.Zero(t, actualResult.Accepted)
			assert.Equal(t, test.errors, actualResult.Errors)
			assert.Equal(t, test.tooLarge, actualResult.TooLarge)
			assert.Equal(t, test.invalid, actualResult.Invalid)
		})
	}
}

func TestHandleStream(t *testing.T) {
	var events []*modelpb.APMEvent
	batchProcessor := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		events = batch.Clone()
		return nil
	})

	payload := strings.Join([]string{
		validMetadata,
		validError,
		validMetricset,
		validSpan,
		validTransaction,
		validLog,
		"", // final newline
	}, "\n")

	p := NewProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    semaphore.NewWeighted(1),
	})
	err := p.HandleStream(
		context.Background(), &modelpb.APMEvent{},
		strings.NewReader(payload), 10, batchProcessor,
		&Result{},
	)
	require.NoError(t, err)

	processors := make([]modelpb.APMEventType, len(events))
	for i, event := range events {
		processors[i] = event.Type()
	}
	assert.Equal(t, []modelpb.APMEventType{
		modelpb.ErrorEventType,
		modelpb.MetricEventType,
		modelpb.SpanEventType,
		modelpb.TransactionEventType,
		modelpb.LogEventType,
	}, processors)
}

func TestHandleStreamRUMv3(t *testing.T) {
	var events []*modelpb.APMEvent
	batchProcessor := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		events = batch.Clone()
		return nil
	})

	payload := strings.Join([]string{
		validRUMv3Metadata,
		validRUMv3Error,
		validRUMv3Transaction,
		"", // final newline
	}, "\n")

	p := NewProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    semaphore.NewWeighted(1),
	})
	var result Result
	err := p.HandleStream(
		context.Background(), &modelpb.APMEvent{},
		strings.NewReader(payload), 10, batchProcessor,
		&result,
	)
	require.NoError(t, err)
	for _, resultErr := range result.Errors {
		require.NoError(t, resultErr)
	}

	processors := make([]modelpb.APMEventType, len(events))
	for i, event := range events {
		processors[i] = event.Type()
	}
	assert.Equal(t, []modelpb.APMEventType{
		modelpb.ErrorEventType,
		modelpb.TransactionEventType,
		modelpb.MetricEventType,
		modelpb.MetricEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
		modelpb.SpanEventType,
	}, processors)
}

func TestHandleStreamBaseEvent(t *testing.T) {
	requestTimestamp := time.Date(2018, 8, 1, 10, 0, 0, 0, time.UTC)

	baseEvent := modelpb.APMEvent{
		Timestamp: modelpb.FromTime(requestTimestamp),
		UserAgent: &modelpb.UserAgent{Original: "rum-2.0"},
		Source:    &modelpb.Source{Ip: modelpb.MustParseIP("192.0.0.1")},
		Client:    &modelpb.Client{Ip: modelpb.MustParseIP("192.0.0.2")}, // X-Forwarded-For
	}

	var events []*modelpb.APMEvent
	batchProcessor := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		events = batch.Clone()
		return nil
	})

	payload := validMetadata + "\n" + validRUMv2Span + "\n"
	p := NewProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    semaphore.NewWeighted(1),
	})
	err := p.HandleStream(
		context.Background(), &baseEvent,
		strings.NewReader(payload), 10, batchProcessor,
		&Result{},
	)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, "rum-2.0", events[0].UserAgent.Original)
	assert.Equal(t, baseEvent.Source, events[0].Source)
	assert.Equal(t, baseEvent.Client, events[0].Client)
	assert.Equal(t, modelpb.FromTime(requestTimestamp.Add(50*time.Millisecond)), events[0].Timestamp) // span's start is "50"
}

func TestLabelLeak(t *testing.T) {
	payload := `{"metadata": {"service": {"name": "testsvc", "environment": "staging", "version": null, "agent": {"name": "python", "version": "6.9.1"}, "language": {"name": "python", "version": "3.10.4"}, "runtime": {"name": "CPython", "version": "3.10.4"}, "framework": {"name": "flask", "version": "2.1.1"}}, "process": {"pid": 2112739, "ppid": 2112738, "argv": ["/home/stuart/workspace/sdh/581/venv/lib/python3.10/site-packages/flask/__main__.py", "run"], "title": null}, "system": {"hostname": "slaptop", "architecture": "x86_64", "platform": "linux"}, "labels": {"ci_commit": "unknown", "numeric": 1}}}
{"transaction": {"id": "88dee29a6571b948", "trace_id": "ba7f5d18ac4c7f39d1ff070c79b2bea5", "name": "GET /withlabels", "type": "request", "duration": 1.6199999999999999, "result": "HTTP 2xx", "timestamp": 1652185276804681, "outcome": "success", "sampled": true, "span_count": {"started": 0, "dropped": 0}, "sample_rate": 1.0, "context": {"request": {"env": {"REMOTE_ADDR": "127.0.0.1", "SERVER_NAME": "127.0.0.1", "SERVER_PORT": "5000"}, "method": "GET", "socket": {"remote_address": "127.0.0.1"}, "cookies": {}, "headers": {"host": "localhost:5000", "user-agent": "curl/7.81.0", "accept": "*/*", "app-os": "Android", "content-type": "application/json; charset=utf-8", "content-length": "29"}, "url": {"full": "http://localhost:5000/withlabels?second_with_labels", "protocol": "http:", "hostname": "localhost", "pathname": "/withlabels", "port": "5000", "search": "?second_with_labels"}}, "response": {"status_code": 200, "headers": {"Content-Type": "application/json", "Content-Length": "14"}}, "tags": {"appOs": "Android", "email_set": "hello@hello.com", "time_set": 1652185276}}}}
{"transaction": {"id": "ba5c6d6c1ab44bd1", "trace_id": "88c0a00431531a80c5ca9a41fe115f41", "name": "GET /nolabels", "type": "request", "duration": 0.652, "result": "HTTP 2xx", "timestamp": 1652185278813952, "outcome": "success", "sampled": true, "span_count": {"started": 0, "dropped": 0}, "sample_rate": 1.0, "context": {"request": {"env": {"REMOTE_ADDR": "127.0.0.1", "SERVER_NAME": "127.0.0.1", "SERVER_PORT": "5000"}, "method": "GET", "socket": {"remote_address": "127.0.0.1"}, "cookies": {}, "headers": {"host": "localhost:5000", "user-agent": "curl/7.81.0", "accept": "*/*"}, "url": {"full": "http://localhost:5000/nolabels?third_no_label", "protocol": "http:", "hostname": "localhost", "pathname": "/nolabels", "port": "5000", "search": "?third_no_label"}}, "response": {"status_code": 200, "headers": {"Content-Type": "text/html; charset=utf-8", "Content-Length": "14"}}, "tags": {}}}}`

	baseEvent := &modelpb.APMEvent{
		Host: &modelpb.Host{
			Ip: []*modelpb.IP{
				modelpb.MustParseIP("192.0.0.1"),
			},
		},
	}

	processed := make(modelpb.Batch, 2)
	batchProcessor := modelpb.ProcessBatchFunc(func(_ context.Context, b *modelpb.Batch) error {
		processed = b.Clone()
		return nil
	})

	p := NewProcessor(Config{
		MaxEventSize: 100 * 1024,
		Semaphore:    semaphore.NewWeighted(1),
	})
	var actualResult Result
	err := p.HandleStream(context.Background(), baseEvent, strings.NewReader(payload), 10, batchProcessor, &actualResult)
	require.NoError(t, err)

	txs := processed
	assert.Len(t, txs, 2)
	// Assert first tx
	assert.Equal(t, modelpb.NumericLabels{
		"time_set": {Value: 1652185276},
		"numeric":  {Global: true, Value: 1},
	}, modelpb.NumericLabels(txs[0].NumericLabels))
	assert.Equal(t, modelpb.Labels{
		"appOs":     {Value: "Android"},
		"email_set": {Value: "hello@hello.com"},
		"ci_commit": {Global: true, Value: "unknown"},
	}, modelpb.Labels(txs[0].Labels))

	// Assert second tx
	assert.Equal(t, modelpb.NumericLabels{"numeric": {Global: true, Value: 1}}, modelpb.NumericLabels(txs[1].NumericLabels))
	assert.Equal(t, modelpb.Labels{"ci_commit": {Global: true, Value: "unknown"}}, modelpb.Labels(txs[1].Labels))
}

type nopBatchProcessor struct{}

func (nopBatchProcessor) ProcessBatch(context.Context, *modelpb.Batch) error {
	return nil
}
