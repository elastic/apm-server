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

package otel_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/processor/otel"
)

func TestConsumerConsumeLogs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
			assert.Empty(t, batch)
			return nil
		}

		consumer := otel.Consumer{Processor: processor}
		logs := plog.NewLogs()
		assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))
	})

	commonEvent := model.APMEvent{
		Processor: model.LogProcessor,
		Agent: model.Agent{
			Name:    "otlp/go",
			Version: "unknown",
		},
		Service: model.Service{
			Name:     "unknown",
			Language: model.Language{Name: "go"},
		},
		Message: "a random log message",
		Event: model.Event{
			Severity: int64(plog.SeverityNumberInfo),
		},
		Log:           model.Log{Level: "Info"},
		Span:          &model.Span{ID: "0200000000000000"},
		Trace:         model.Trace{ID: "01000000000000000000000000000000"},
		Labels:        model.Labels{},
		NumericLabels: model.NumericLabels{},
	}
	test := func(name string, body interface{}, expectedMessage string) {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			logs.ResourceLogs().At(0).Resource().Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, "go")
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			newLogRecord(body).CopyTo(scopeLogs.LogRecords().AppendEmpty())

			var processed model.Batch
			var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
				if processed != nil {
					panic("already processes batch")
				}
				processed = *batch
				assert.NotNil(t, processed[0].Timestamp)
				processed[0].Timestamp = time.Time{}
				return nil
			}
			consumer := otel.Consumer{Processor: processor}
			assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))

			expected := commonEvent
			expected.Message = expectedMessage
			assert.Equal(t, model.Batch{expected}, processed)
		})
	}
	test("string_body", "a random log message", "a random log message")
	test("int_body", 1234, "1234")
	test("float_body", 1234.1234, "1234.1234")
	test("bool_body", true, "true")
	// TODO(marclop): How to test map body
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

	var processed model.Batch
	var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = *batch
		assert.NotNil(t, processed[0].Timestamp)
		processed[0].Timestamp = time.Time{}
		return nil
	}
	consumer := otel.Consumer{Processor: processor}
	assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))

	assert.Len(t, processed, 2)
	assert.Equal(t, model.Labels{"key0": {Global: true, Value: "zero"}, "key1": {Value: "one"}}, processed[0].Labels)
	assert.Empty(t, processed[0].NumericLabels)
	out := cmp.Diff(model.APMEvent{
		Service: model.Service{
			Name: "unknown",
			Language: model.Language{
				Name: "java",
			},
		},
		Agent: model.Agent{
			Name:    "otlp/java",
			Version: "unknown",
		},
		Event: model.Event{
			Severity: int64(plog.SeverityNumberInfo),
		},
		Labels:        model.Labels{"key0": {Global: true, Value: "zero"}},
		NumericLabels: model.NumericLabels{},
		Processor:     model.ErrorProcessor,
		Message:       "bar",
		Trace:         model.Trace{ID: "01000000000000000000000000000000"},
		Span:          &model.Span{ID: "0200000000000000"},
		Log: model.Log{
			Level: "Info",
		},
		Error: &model.Error{
			Type: "crash",
			Exception: &model.Exception{
				Type:    "HighLevelException",
				Message: "MidLevelException: LowLevelException",
				Handled: newBool(true),
				Stacktrace: []*model.StacktraceFrame{{
					Classname: "Junk",
					Function:  "a",
					Filename:  "Junk.java",
					Lineno:    newInt(13),
				}, {
					Classname: "Junk",
					Function:  "main",
					Filename:  "Junk.java",
					Lineno:    newInt(4),
				}},
				Cause: []model.Exception{{
					Message: "MidLevelException: LowLevelException",
					Handled: newBool(true),
					Stacktrace: []*model.StacktraceFrame{{
						Classname: "Junk",
						Function:  "c",
						Filename:  "Junk.java",
						Lineno:    newInt(23),
					}, {
						Classname: "Junk",
						Function:  "b",
						Filename:  "Junk.java",
						Lineno:    newInt(17),
					}, {
						Classname: "Junk",
						Function:  "a",
						Filename:  "Junk.java",
						Lineno:    newInt(11),
					}, {
						Classname: "Junk",
						Function:  "main",
						Filename:  "Junk.java",
						Lineno:    newInt(4),
					}},
					Cause: []model.Exception{{
						Message: "LowLevelException",
						Handled: newBool(true),
						Stacktrace: []*model.StacktraceFrame{{
							Classname: "Junk",
							Function:  "e",
							Filename:  "Junk.java",
							Lineno:    newInt(37),
						}, {
							Classname: "Junk",
							Function:  "d",
							Filename:  "Junk.java",
							Lineno:    newInt(34),
						}, {
							Classname: "Junk",
							Function:  "c",
							Filename:  "Junk.java",
							Lineno:    newInt(21),
						}, {
							Classname: "Junk",
							Function:  "b",
							Filename:  "Junk.java",
							Lineno:    newInt(17),
						}, {
							Classname: "Junk",
							Function:  "a",
							Filename:  "Junk.java",
							Lineno:    newInt(11),
						}, {
							Classname: "Junk",
							Function:  "main",
							Filename:  "Junk.java",
							Lineno:    newInt(4),
						}},
					}},
				}},
			},
		},
	}, processed[1], cmpopts.IgnoreFields(model.APMEvent{}, "Error.ID", "Timestamp"), cmpopts.IgnoreTypes(netip.Addr{}))
	assert.Empty(t, out)
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

	var processed model.Batch
	var processor model.ProcessBatchFunc = func(_ context.Context, batch *model.Batch) error {
		if processed != nil {
			panic("already processes batch")
		}
		processed = *batch
		assert.NotNil(t, processed[0].Timestamp)
		processed[0].Timestamp = time.Time{}
		return nil
	}
	consumer := otel.Consumer{Processor: processor}
	assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))

	assert.Len(t, processed, 3)
	assert.Equal(t, model.Labels{"key0": {Global: true, Value: "zero"}, "key1": {Value: "one"}}, processed[0].Labels)
	assert.Empty(t, processed[0].NumericLabels)
	assert.Equal(t, model.Labels{"key0": {Global: true, Value: "zero"}}, processed[1].Labels)
	assert.Equal(t, model.NumericLabels{"key2": {Value: 2}}, processed[1].NumericLabels)
	assert.Equal(t, model.Labels{"key0": {Global: true, Value: "zero"}, "key3": {Value: "three"}}, processed[2].Labels)
	assert.Equal(t, model.NumericLabels{"key4": {Value: 4}}, processed[2].NumericLabels)
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
