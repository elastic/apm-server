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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"

	"github.com/elastic/apm-server/internal/model"
)

func TestEncodeSpanEventsNonExceptions(t *testing.T) {
	nonExceptionEvent := ptrace.NewSpanEvent()
	nonExceptionEvent.SetName("not_exception")

	incompleteExceptionEvent := ptrace.NewSpanEvent()
	incompleteExceptionEvent.SetName("exception")
	incompleteExceptionEvent.Attributes().InsertString(
		// At least one of exception.message and exception.type is required.
		semconv.AttributeExceptionStacktrace, "stacktrace",
	)

	_, events := transformTransactionSpanEvents(t, "java", nonExceptionEvent, incompleteExceptionEvent)
	require.Len(t, events, 2)
	assert.Equal(t, model.LogProcessor, events[0].Processor)
	assert.Equal(t, model.LogProcessor, events[1].Processor)
}

func TestEncodeSpanEventsJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent1 := ptrace.NewSpanEvent()
	exceptionEvent1.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent1.SetName("exception")
	exceptionEvent1.Attributes().InsertString("exception.type", "java.net.ConnectException.OSError")
	exceptionEvent1.Attributes().InsertString("exception.message", "Division by zero")
	exceptionEvent1.Attributes().InsertBool("exception.escaped", true)
	exceptionEvent1.Attributes().InsertString("exception.stacktrace", `
Exception in thread "main" java.lang.RuntimeException: Test exception
	at com.example.GenerateTrace.methodB(GenerateTrace.java:13)
	at com.example.GenerateTrace.methodA(GenerateTrace.java:9)
	at com.example.GenerateTrace.main(GenerateTrace.java:5)
	at com.foo.loader/foo@9.0/com.foo.Main.run(Main.java)
	at com.foo.loader//com.foo.bar.App.run(App.java:12)
	at java.base/java.lang.Thread.run(Unknown Source)
`[1:])

	exceptionEvent2 := ptrace.NewSpanEvent()
	exceptionEvent2.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent2.SetName("exception")
	exceptionEvent2.Attributes().InsertString("exception.type", "HighLevelException")
	exceptionEvent2.Attributes().InsertString("exception.message", "MidLevelException: LowLevelException")
	exceptionEvent2.Attributes().InsertString("exception.stacktrace", `
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

	service, agent := languageOnlyMetadata("java")
	transactionEvent, errorEvents := transformTransactionSpanEvents(t, "java", exceptionEvent1, exceptionEvent2)
	assert.Equal(t, []model.APMEvent{{
		Service:       service,
		Agent:         agent,
		Timestamp:     timestamp,
		Labels:        model.Labels{},
		NumericLabels: model.NumericLabels{},
		Processor:     model.ErrorProcessor,
		Trace:         transactionEvent.Trace,
		Parent:        model.Parent{ID: transactionEvent.Transaction.ID},
		Transaction: &model.Transaction{
			ID:      transactionEvent.Transaction.ID,
			Type:    transactionEvent.Transaction.Type,
			Sampled: true,
		},
		Error: &model.Error{
			Exception: &model.Exception{
				Type:    "java.net.ConnectException.OSError",
				Message: "Division by zero",
				Handled: newBool(false),
				Stacktrace: []*model.StacktraceFrame{{
					Classname: "com.example.GenerateTrace",
					Function:  "methodB",
					Filename:  "GenerateTrace.java",
					Lineno:    newInt(13),
				}, {
					Classname: "com.example.GenerateTrace",
					Function:  "methodA",
					Filename:  "GenerateTrace.java",
					Lineno:    newInt(9),
				}, {
					Classname: "com.example.GenerateTrace",
					Function:  "main",
					Filename:  "GenerateTrace.java",
					Lineno:    newInt(5),
				}, {
					Module:    "foo@9.0",
					Classname: "com.foo.Main",
					Function:  "run",
					Filename:  "Main.java",
				}, {
					Classname: "com.foo.bar.App",
					Function:  "run",
					Filename:  "App.java",
					Lineno:    newInt(12),
				}, {
					Module:    "java.base",
					Classname: "java.lang.Thread",
					Function:  "run",
					Filename:  "Unknown Source",
				}},
			},
		},
	}, {
		Service:       service,
		Agent:         agent,
		Timestamp:     timestamp,
		Labels:        model.Labels{},
		NumericLabels: model.NumericLabels{},
		Processor:     model.ErrorProcessor,
		Trace:         transactionEvent.Trace,
		Parent:        model.Parent{ID: transactionEvent.Transaction.ID},
		Transaction: &model.Transaction{
			ID:      transactionEvent.Transaction.ID,
			Type:    transactionEvent.Transaction.Type,
			Sampled: true,
		},
		Error: &model.Error{
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
	}}, errorEvents)
}

func TestEncodeSpanEventsJavaExceptionsUnparsedStacktrace(t *testing.T) {
	stacktraces := []string{
		// Unexpected prefix.
		"abc\ndef",

		// "... N more" with no preceding exception.
		"abc\n... 1 more",

		// "... N more" where N is greater than the number of stack
		// frames in the enclosing exception.
		`ignored message
	at Class.method(Class.java:1)
Caused by: something else
	at Class.method(Class.java:2)
	... 2 more`,

		// "... N more" where N is not a sequence of digits.
		`abc
	at Class.method(Class.java:1)
Caused by: whatever
	at Class.method(Class.java:2)
	... lots more`,

		// "at <location>" where <location> is invalid.
		`abc
	at the movies`,
	}

	var events []ptrace.SpanEvent
	for _, stacktrace := range stacktraces {
		event := ptrace.NewSpanEvent()
		event.SetName("exception")
		event.Attributes().InsertString("exception.type", "ExceptionType")
		event.Attributes().InsertString("exception.stacktrace", stacktrace)
		events = append(events, event)
	}

	_, errorEvents := transformTransactionSpanEvents(t, "java", events...)
	require.Len(t, errorEvents, len(stacktraces))

	for i, event := range errorEvents {
		assert.Empty(t, event.Error.Exception.Stacktrace)
		assert.Equal(t, stacktraces[i], event.Error.StackTrace)
	}
}

func TestEncodeSpanEventsNonJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent := ptrace.NewSpanEvent()
	exceptionEvent.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent.SetName("exception")
	exceptionEvent.Attributes().InsertString("exception.type", "the_type")
	exceptionEvent.Attributes().InsertString("exception.message", "the_message")
	exceptionEvent.Attributes().InsertString("exception.stacktrace", "the_stacktrace")

	// For languages where we do not explicitly parse the stacktrace,
	// the raw stacktrace is stored as error.stack_trace.
	transactionEvent, errorEvents := transformTransactionSpanEvents(t, "COBOL", exceptionEvent)
	require.Len(t, errorEvents, 1)

	service, agent := languageOnlyMetadata("COBOL")
	assert.Equal(t, model.APMEvent{
		Service:       service,
		Agent:         agent,
		Timestamp:     timestamp,
		Labels:        model.Labels{},
		NumericLabels: model.NumericLabels{},
		Processor:     model.ErrorProcessor,
		Trace:         transactionEvent.Trace,
		Parent:        model.Parent{ID: transactionEvent.Transaction.ID},
		Transaction: &model.Transaction{
			ID:      transactionEvent.Transaction.ID,
			Type:    transactionEvent.Transaction.Type,
			Sampled: true,
		},
		Error: &model.Error{
			Exception: &model.Exception{
				Type:    "the_type",
				Message: "the_message",
				Handled: newBool(true),
			},
			StackTrace: "the_stacktrace",
		},
	}, errorEvents[0])
}

func languageOnlyMetadata(language string) (model.Service, model.Agent) {
	service := model.Service{
		Name:     "unknown",
		Language: model.Language{Name: language},
	}
	agent := model.Agent{
		Name:    "otlp/" + language,
		Version: "unknown",
	}
	return service, agent
}
