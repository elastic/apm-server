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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlp_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestConsumer_ConsumeLogs_Interface(t *testing.T) {
	var _ consumer.Logs = otlp.NewConsumer(otlp.ConsumerConfig{})
}

func TestConsumerConsumeLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
			assert.Empty(t, batch)
			return nil
		}

		consumer := otlp.NewConsumer(otlp.ConsumerConfig{
			Processor: processor,
			Semaphore: semaphore.NewWeighted(100),
		})
		logs := plog.NewLogs()
		result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
		assert.NoError(t, err)
		assert.Equal(t, otlp.ConsumeLogsResult{}, result)
	})

	commonEvent := modelpb.APMEvent{
		Agent: &modelpb.Agent{
			Name:    "otlp/go",
			Version: "unknown",
		},
		Service: &modelpb.Service{
			Name:     "unknown",
			Language: &modelpb.Language{Name: "go"},
		},
		Message: "a random log message",
		Event: &modelpb.Event{
			Severity: uint64(plog.SeverityNumberInfo),
		},
		Log:           &modelpb.Log{Level: "Info"},
		Span:          &modelpb.Span{Id: "0200000000000000"},
		Trace:         &modelpb.Trace{Id: "01000000000000000000000000000000"},
		Labels:        modelpb.Labels{},
		NumericLabels: modelpb.NumericLabels{},
	}

	for _, tt := range []struct {
		name  string
		body  any
		attrs map[string]string

		expectedEvent *modelpb.APMEvent
	}{
		{
			name: "with a string body",
			body: "a random log message",

			expectedEvent: &modelpb.APMEvent{
				Message: "a random log message",
			},
		},
		{
			name: "with an int body",
			body: 1234,

			expectedEvent: &modelpb.APMEvent{
				Message: "1234",
			},
		},
		{
			name: "with a float body",
			body: 1234.1234,

			expectedEvent: &modelpb.APMEvent{
				Message: "1234.1234",
			},
		},
		{
			name: "with a bool body",
			body: true,

			expectedEvent: &modelpb.APMEvent{
				Message: "true",
			},
		},
		{
			name: "with an event name",
			body: "a log message",

			attrs: map[string]string{
				"event.name": "hello_world",
			},

			expectedEvent: &modelpb.APMEvent{
				Message: "a log message",
			},
		},
		{
			name: "with an event domain",
			body: "a log message",

			attrs: map[string]string{
				"event.domain": "device",
			},

			expectedEvent: &modelpb.APMEvent{
				Message: "a log message",
			},
		},
		{
			name: "with a session id",
			body: "a log message",

			attrs: map[string]string{
				"session.id": "abcd",
			},

			expectedEvent: &modelpb.APMEvent{
				Session: &modelpb.Session{
					Id: "abcd",
				},
				Message: "a log message",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			logs.ResourceLogs().At(0).Resource().Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, "go")
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			record := newLogRecord(tt.body)

			for k, v := range tt.attrs {
				record.Attributes().PutStr(k, v)
			}
			record.CopyTo(scopeLogs.LogRecords().AppendEmpty())

			var processed modelpb.Batch
			var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
				if processed != nil {
					panic("already processes batch")
				}
				processed = batch.Clone()
				assert.NotZero(t, processed[0].Timestamp)
				processed[0].Timestamp = 0
				return nil
			}
			consumer := otlp.NewConsumer(otlp.ConsumerConfig{
				Processor: processor,
				Semaphore: semaphore.NewWeighted(100),
			})
			result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
			assert.NoError(t, err)
			assert.Equal(t, otlp.ConsumeLogsResult{}, result)

			now := modelpb.FromTime(time.Now())
			for _, e := range processed {
				assert.InDelta(t, now, e.Event.Received, float64((2 * time.Second).Nanoseconds()))
				e.Event.Received = 0
			}

			expected := proto.Clone(&commonEvent).(*modelpb.APMEvent)
			expected.Message = tt.expectedEvent.Message
			if tt.expectedEvent.Session != nil {
				expected.Session = tt.expectedEvent.Session
			}

			assert.Empty(t, cmp.Diff(modelpb.Batch{expected}, processed, protocmp.Transform()))
		})
	}
	// TODO(marclop): How to test map body
}

func TestConsumeLogsSemaphore(t *testing.T) {
	logs := plog.NewLogs()
	var batches []*modelpb.Batch

	doneCh := make(chan struct{})
	recorder := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		<-doneCh
		batchCopy := batch.Clone()
		batches = append(batches, &batchCopy)
		return nil
	})
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: recorder,
		Semaphore: semaphore.NewWeighted(1),
	})

	startCh := make(chan struct{})
	go func() {
		close(startCh)
		_, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
		assert.NoError(t, err)
	}()

	<-startCh
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := consumer.ConsumeLogsWithResult(ctx, logs)
	assert.Equal(t, err.Error(), "context deadline exceeded")
	close(doneCh)

	_, err = consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
}

func TestConsumerConsumeLogsException(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
	resourceAttrs.PutStr("key0", "zero")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	record1 := newLogRecord("foo")
	record1.Attributes().PutStr("key1", "one")
	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	record2 := newLogRecord("bar")
	record2.Attributes().PutStr("event.name", "crash")
	record2.Attributes().PutStr("event.domain", "device")
	record2.Attributes().PutStr("exception.type", "HighLevelException")
	record2.Attributes().PutStr("exception.message", "MidLevelException: LowLevelException")
	record2.Attributes().PutStr("exception.stacktrace", `
HighLevelException: MidLevelException: LowLevelException
	at Junk.a(Junk.java:13)
	at Junk.main(Junk.java:4)
Caused by: MidLevelException: LowLevelException
	at Junk.c(Junk.java:23)
	at Junk.b(Junk.java:17)
	at Junk.a(Junk.java:11)
	... 1 more
	Suppressed: java.lang.ArithmeticException: / by zero
		at Junk.c(Junk.java:25)
		... 3 more
Caused by: LowLevelException
	at Junk.e(Junk.java:37)
	at Junk.d(Junk.java:34)
	at Junk.c(Junk.java:21)
	... 3 more`[1:])
	record2.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		assert.NotZero(t, processed[0].Timestamp)
		processed[0].Timestamp = 0
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)

	now := modelpb.FromTime(time.Now())
	for _, e := range processed {
		assert.InDelta(t, now, e.Event.Received, float64((2 * time.Second).Nanoseconds()))
		e.Event.Received = 0
	}

	assert.Len(t, processed, 2)
	assert.Equal(t, modelpb.Labels{"key0": {Global: true, Value: "zero"}, "key1": {Value: "one"}}, modelpb.Labels(processed[0].Labels))
	assert.Empty(t, processed[0].NumericLabels)
	processed[1].Timestamp = 0
	out := cmp.Diff(&modelpb.APMEvent{
		Service: &modelpb.Service{
			Name: "unknown",
			Language: &modelpb.Language{
				Name: "java",
			},
		},
		Agent: &modelpb.Agent{
			Name:    "otlp/java",
			Version: "unknown",
		},
		Event: &modelpb.Event{
			Severity: uint64(plog.SeverityNumberInfo),
			Kind:     "event",
			Category: "device",
			Type:     "error",
		},
		Labels:        modelpb.Labels{"key0": {Global: true, Value: "zero"}},
		NumericLabels: modelpb.NumericLabels{},
		Message:       "bar",
		Trace:         &modelpb.Trace{Id: "01000000000000000000000000000000"},
		Span:          &modelpb.Span{Id: "0200000000000000"},
		Log: &modelpb.Log{
			Level: "Info",
		},
		Error: &modelpb.Error{
			Type: "crash",
			Exception: &modelpb.Exception{
				Type:    "HighLevelException",
				Message: "MidLevelException: LowLevelException",
				Handled: newBool(true),
				Stacktrace: []*modelpb.StacktraceFrame{{
					Classname: "Junk",
					Function:  "a",
					Filename:  "Junk.java",
					Lineno:    newUint32(13),
				}, {
					Classname: "Junk",
					Function:  "main",
					Filename:  "Junk.java",
					Lineno:    newUint32(4),
				}},
				Cause: []*modelpb.Exception{{
					Message: "MidLevelException: LowLevelException",
					Handled: newBool(true),
					Stacktrace: []*modelpb.StacktraceFrame{{
						Classname: "Junk",
						Function:  "c",
						Filename:  "Junk.java",
						Lineno:    newUint32(23),
					}, {
						Classname: "Junk",
						Function:  "b",
						Filename:  "Junk.java",
						Lineno:    newUint32(17),
					}, {
						Classname: "Junk",
						Function:  "a",
						Filename:  "Junk.java",
						Lineno:    newUint32(11),
					}, {
						Classname: "Junk",
						Function:  "main",
						Filename:  "Junk.java",
						Lineno:    newUint32(4),
					}},
					Cause: []*modelpb.Exception{{
						Message: "LowLevelException",
						Handled: newBool(true),
						Stacktrace: []*modelpb.StacktraceFrame{{
							Classname: "Junk",
							Function:  "e",
							Filename:  "Junk.java",
							Lineno:    newUint32(37),
						}, {
							Classname: "Junk",
							Function:  "d",
							Filename:  "Junk.java",
							Lineno:    newUint32(34),
						}, {
							Classname: "Junk",
							Function:  "c",
							Filename:  "Junk.java",
							Lineno:    newUint32(21),
						}, {
							Classname: "Junk",
							Function:  "b",
							Filename:  "Junk.java",
							Lineno:    newUint32(17),
						}, {
							Classname: "Junk",
							Function:  "a",
							Filename:  "Junk.java",
							Lineno:    newUint32(11),
						}, {
							Classname: "Junk",
							Function:  "main",
							Filename:  "Junk.java",
							Lineno:    newUint32(4),
						}},
					}},
				}},
			},
		},
	}, processed[1],
		protocmp.Transform(),
		protocmp.IgnoreFields(&modelpb.Error{}, "id"),
	)
	assert.Empty(t, out)
}

func TestConsumerConsumeOTelEventLogs(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "swift")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	record1 := newLogRecord("") // no log body
	record1.Attributes().PutStr("event.domain", "device")
	record1.Attributes().PutStr("event.name", "MyEvent")
	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	record2 := newLogRecord("") // no log body
	record2.Attributes().PutStr("event.name", "device.MyEvent2")
	record2.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	processed := processLogEvents(t, logs)

	assert.Len(t, processed, 2)
	expected := []struct {
		kind     string
		category string
		action   string
	}{
		{kind: "event", category: "device", action: "MyEvent"},
		{kind: "event", category: "device", action: "MyEvent2"},
	}
	for i, item := range processed {
		expectedValues := expected[i]
		assert.Equal(t, expectedValues.kind, item.Event.Kind)
		assert.Equal(t, expectedValues.category, item.Event.Category)
		assert.Equal(t, expectedValues.action, item.Event.Action)
	}
}

func TestConsumerConsumeLogsExceptionAsEvents(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
	resourceAttrs.PutStr("key0", "zero")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	record1 := newLogRecord("bar")
	record1.Attributes().PutStr("event.name", "crash")
	record1.Attributes().PutStr("event.domain", "device")
	record1.Attributes().PutStr("exception.type", "HighLevelException")
	record1.Attributes().PutStr("exception.message", "MidLevelException: LowLevelException")
	record1.Attributes().PutStr("exception.stacktrace", "HighLevelException: MidLevelException: LowLevelException")
	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	record2 := newLogRecord("bar")
	record2.Attributes().PutStr("event.name", "device.crash")
	record2.Attributes().PutStr("exception.type", "HighLevelException")
	record2.Attributes().PutStr("exception.message", "MidLevelException: LowLevelException2")
	record2.Attributes().PutStr("exception.stacktrace", "HighLevelException: MidLevelException: LowLevelException")
	record2.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	processed := processLogEvents(t, logs)

	assert.Len(t, processed, 2)
	expected := []struct {
		kind      string
		category  string
		eventType string
		errorType string
	}{
		{kind: "event", category: "device", eventType: "error", errorType: "crash"},
		{kind: "event", category: "device", eventType: "error", errorType: "crash"},
	}
	for i, item := range processed {
		expectedValues := expected[i]
		assert.Equal(t, expectedValues.kind, item.Event.Kind)
		assert.Equal(t, expectedValues.category, item.Event.Category)
		assert.Equal(t, expectedValues.eventType, item.Event.Type)
		assert.Equal(t, expectedValues.errorType, item.Error.Type)
	}
}

func processLogEvents(t *testing.T, logs plog.Logs) modelpb.Batch {
	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processed batch")
		}
		processed = batch.Clone()
		assert.NotZero(t, processed[0].Timestamp)
		processed[0].Timestamp = 0
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)
	return processed
}

func TestConsumerConsumeLogsDataStream(t *testing.T) {
	for _, tc := range []struct {
		resourceDataStreamDataset   string
		resourceDataStreamNamespace string
		scopeDataStreamDataset      string
		scopeDataStreamNamespace    string
		recordDataStreamDataset     string
		recordDataStreamNamespace   string

		expectedDataStreamDataset   string
		expectedDataStreamNamespace string
	}{
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			scopeDataStreamDataset:      "3",
			scopeDataStreamNamespace:    "4",
			recordDataStreamDataset:     "5",
			recordDataStreamNamespace:   "6",
			expectedDataStreamDataset:   "5",
			expectedDataStreamNamespace: "6",
		},
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			scopeDataStreamDataset:      "3",
			scopeDataStreamNamespace:    "4",
			expectedDataStreamDataset:   "3",
			expectedDataStreamNamespace: "4",
		},
		{
			resourceDataStreamDataset:   "1",
			resourceDataStreamNamespace: "2",
			expectedDataStreamDataset:   "1",
			expectedDataStreamNamespace: "2",
		},
	} {
		tcName := fmt.Sprintf("%s,%s", tc.expectedDataStreamDataset, tc.expectedDataStreamNamespace)
		t.Run(tcName, func(t *testing.T) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
			if tc.resourceDataStreamDataset != "" {
				resourceAttrs.PutStr("data_stream.dataset", tc.resourceDataStreamDataset)
			}
			if tc.resourceDataStreamNamespace != "" {
				resourceAttrs.PutStr("data_stream.namespace", tc.resourceDataStreamNamespace)
			}

			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			scopeAttrs := resourceLogs.ScopeLogs().At(0).Scope().Attributes()
			if tc.scopeDataStreamDataset != "" {
				scopeAttrs.PutStr("data_stream.dataset", tc.scopeDataStreamDataset)
			}
			if tc.scopeDataStreamNamespace != "" {
				scopeAttrs.PutStr("data_stream.namespace", tc.scopeDataStreamNamespace)
			}

			record1 := newLogRecord("") // no log body
			record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())
			recordAttrs := scopeLogs.LogRecords().At(0).Attributes()
			if tc.recordDataStreamDataset != "" {
				recordAttrs.PutStr("data_stream.dataset", tc.recordDataStreamDataset)
			}
			if tc.recordDataStreamNamespace != "" {
				recordAttrs.PutStr("data_stream.namespace", tc.recordDataStreamNamespace)
			}

			var processed modelpb.Batch
			var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
				if processed != nil {
					panic("already processes batch")
				}
				processed = batch.Clone()
				assert.NotZero(t, processed[0].Timestamp)
				processed[0].Timestamp = 0
				return nil
			}
			consumer := otlp.NewConsumer(otlp.ConsumerConfig{
				Processor: processor,
				Semaphore: semaphore.NewWeighted(100),
			})
			result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
			assert.NoError(t, err)
			assert.Equal(t, otlp.ConsumeLogsResult{}, result)

			assert.Len(t, processed, 1)
			assert.Equal(t, tc.expectedDataStreamDataset, processed[0].DataStream.Dataset)
			assert.Equal(t, tc.expectedDataStreamNamespace, processed[0].DataStream.Namespace)
		})
	}
}

func TestConsumerConsumeOTelLogsWithTimestamp(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	timestamp := pcommon.NewTimestampFromTime(time.UnixMilli(946684800000))

	record1 := newLogRecord("") // no log body
	record1.SetTimestamp(timestamp)

	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)

	assert.Len(t, processed, 1)
	assert.Equal(t, int(timestamp.AsTime().UnixNano()), int(processed[0].Timestamp))
}

func TestConsumerConsumeOTelLogsWithoutTimestamp(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	timestamp := pcommon.NewTimestampFromTime(time.UnixMilli(0))

	record1 := newLogRecord("") // no log body
	record1.SetTimestamp(0)

	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)

	assert.Len(t, processed, 1)
	assert.Equal(t, int(timestamp.AsTime().UnixNano()), int(processed[0].Timestamp))
}

func TestConsumerConsumeOTelLogsWithObservedTimestampWithoutTimestamp(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "java")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	observedTimestamp := pcommon.NewTimestampFromTime(time.UnixMilli(946684800000))

	record1 := newLogRecord("") // no log body
	record1.SetTimestamp(0)
	record1.SetObservedTimestamp(observedTimestamp)

	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)

	assert.Len(t, processed, 1)
	assert.Equal(t, int(observedTimestamp.AsTime().UnixNano()), int(processed[0].Timestamp))
}

func TestConsumerConsumeLogsLabels(t *testing.T) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := logs.ResourceLogs().At(0).Resource().Attributes()
	resourceAttrs.PutStr(semconv.AttributeTelemetrySDKLanguage, "go")
	resourceAttrs.PutStr("key0", "zero")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	record1 := newLogRecord("whatever")
	record1.Attributes().PutStr("key1", "one")
	record1.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	record2 := newLogRecord("andever")
	record2.Attributes().PutDouble("key2", 2)
	record2.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	record3 := newLogRecord("amen")
	record3.Attributes().PutStr("key3", "three")
	record3.Attributes().PutInt("key4", 4)
	record3.CopyTo(scopeLogs.LogRecords().AppendEmpty())

	var processed modelpb.Batch
	var processor modelpb.ProcessBatchFunc = func(_ context.Context, batch *modelpb.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = batch.Clone()
		assert.NotZero(t, processed[0].Timestamp)
		processed[0].Timestamp = 0
		return nil
	}
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Semaphore: semaphore.NewWeighted(100),
	})
	result, err := consumer.ConsumeLogsWithResult(context.Background(), logs)
	assert.NoError(t, err)
	assert.Equal(t, otlp.ConsumeLogsResult{}, result)

	assert.Len(t, processed, 3)
	assert.Equal(t, modelpb.Labels{"key0": {Global: true, Value: "zero"}, "key1": {Value: "one"}}, modelpb.Labels(processed[0].Labels))
	assert.Empty(t, processed[0].NumericLabels)
	assert.Equal(t, modelpb.Labels{"key0": {Global: true, Value: "zero"}}, modelpb.Labels(processed[1].Labels))
	assert.Equal(t, modelpb.NumericLabels{"key2": {Value: 2}}, modelpb.NumericLabels(processed[1].NumericLabels))
	assert.Equal(t, modelpb.Labels{"key0": {Global: true, Value: "zero"}, "key3": {Value: "three"}}, modelpb.Labels(processed[2].Labels))
	assert.Equal(t, modelpb.NumericLabels{"key4": {Value: 4}}, modelpb.NumericLabels(processed[2].NumericLabels))
}

func newLogRecord(body interface{}) plog.LogRecord {
	otelLogRecord := plog.NewLogRecord()
	otelLogRecord.SetTraceID(pcommon.TraceID{1})
	otelLogRecord.SetSpanID(pcommon.SpanID{2})
	otelLogRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	otelLogRecord.SetSeverityText("Info")
	otelLogRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	switch b := body.(type) {
	case string:
		otelLogRecord.Body().SetStr(b)
	case int:
		otelLogRecord.Body().SetInt(int64(b))
	case float64:
		otelLogRecord.Body().SetDouble(b)
	case bool:
		otelLogRecord.Body().SetBool(b)
		// case map[string]string:
		// TODO(marclop) figure out how to set the body since it cannot be set
		// as a map.
		// otelLogRecord.Body()
	}
	return otelLogRecord
}
