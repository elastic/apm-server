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
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/elastic/apm-server/model"
)

func TestEncodeSpanEventsNonExceptions(t *testing.T) {
	nonExceptionEvent := pdata.NewSpanEvent()
	nonExceptionEvent.SetName("not_exception")

	incompleteExceptionEvent := pdata.NewSpanEvent()
	incompleteExceptionEvent.SetName("exception")
	incompleteExceptionEvent.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		// At least one of exception.message and exception.type is required.
		semconv.AttributeExceptionStacktrace: pdata.NewAttributeValueString("stacktrace"),
	})

	_, errors := transformTransactionSpanEvents(t, "java", nonExceptionEvent, incompleteExceptionEvent)
	require.Empty(t, errors)
}

func TestEncodeSpanEventsJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent1 := pdata.NewSpanEvent()
	exceptionEvent1.SetTimestamp(pdata.TimestampFromTime(timestamp))
	exceptionEvent1.SetName("exception")
	exceptionEvent1.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":    pdata.NewAttributeValueString("java.net.ConnectException.OSError"),
		"exception.message": pdata.NewAttributeValueString("Division by zero"),
		"exception.escaped": pdata.NewAttributeValueBool(true),
		"exception.stacktrace": pdata.NewAttributeValueString(`
Exception in thread "main" java.lang.RuntimeException: Test exception
	at com.example.GenerateTrace.methodB(GenerateTrace.java:13)
	at com.example.GenerateTrace.methodA(GenerateTrace.java:9)
	at com.example.GenerateTrace.main(GenerateTrace.java:5)
	at com.foo.loader/foo@9.0/com.foo.Main.run(Main.java)
	at com.foo.loader//com.foo.bar.App.run(App.java:12)
	at java.base/java.lang.Thread.run(Unknown Source)
`[1:],
		),
	})
	exceptionEvent2 := pdata.NewSpanEvent()
	exceptionEvent2.SetTimestamp(pdata.TimestampFromTime(timestamp))
	exceptionEvent2.SetName("exception")
	exceptionEvent2.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":    pdata.NewAttributeValueString("HighLevelException"),
		"exception.message": pdata.NewAttributeValueString("MidLevelException: LowLevelException"),
		"exception.stacktrace": pdata.NewAttributeValueString(`
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
	... 3 more`[1:],
		),
	})

	service, agent := languageOnlyMetadata("java")
	transactionEvent, errorEvents := transformTransactionSpanEvents(t, "java", exceptionEvent1, exceptionEvent2)
	assert.Equal(t, []model.APMEvent{{
		Service:   service,
		Agent:     agent,
		Timestamp: timestamp,
		Processor: model.ErrorProcessor,
		Trace:     transactionEvent.Trace,
		Error: &model.Error{
			ParentID:           transactionEvent.Transaction.ID,
			TransactionID:      transactionEvent.Transaction.ID,
			TransactionType:    transactionEvent.Transaction.Type,
			TransactionSampled: newBool(true),
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
		Service:   service,
		Agent:     agent,
		Timestamp: timestamp,
		Processor: model.ErrorProcessor,
		Trace:     transactionEvent.Trace,
		Error: &model.Error{
			ParentID:           transactionEvent.Transaction.ID,
			TransactionID:      transactionEvent.Transaction.ID,
			TransactionType:    transactionEvent.Transaction.Type,
			TransactionSampled: newBool(true),
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

	var events []pdata.SpanEvent
	for _, stacktrace := range stacktraces {
		event := pdata.NewSpanEvent()
		event.SetName("exception")
		event.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			"exception.type":       pdata.NewAttributeValueString("ExceptionType"),
			"exception.stacktrace": pdata.NewAttributeValueString(stacktrace),
		})
		events = append(events, event)
	}

	_, errorEvents := transformTransactionSpanEvents(t, "java", events...)
	require.Len(t, errorEvents, len(stacktraces))

	for i, event := range errorEvents {
		assert.Empty(t, event.Error.Exception.Stacktrace)
		assert.Equal(t, map[string]interface{}{"stacktrace": stacktraces[i]}, event.Error.Exception.Attributes)
	}
}

func TestEncodeSpanEventsNonJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent := pdata.NewSpanEvent()
	exceptionEvent.SetTimestamp(pdata.TimestampFromTime(timestamp))
	exceptionEvent.SetName("exception")
	exceptionEvent.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":       pdata.NewAttributeValueString("the_type"),
		"exception.message":    pdata.NewAttributeValueString("the_message"),
		"exception.stacktrace": pdata.NewAttributeValueString("the_stacktrace"),
	})

	// For languages where we do not explicitly parse the stacktrace,
	// the raw stacktrace is stored as an attribute on the exception.
	transactionEvent, errorEvents := transformTransactionSpanEvents(t, "COBOL", exceptionEvent)
	require.Len(t, errorEvents, 1)

	service, agent := languageOnlyMetadata("COBOL")
	assert.Equal(t, model.APMEvent{
		Service:   service,
		Agent:     agent,
		Timestamp: timestamp,
		Processor: model.ErrorProcessor,
		Trace:     transactionEvent.Trace,
		Error: &model.Error{
			ParentID:           transactionEvent.Transaction.ID,
			TransactionID:      transactionEvent.Transaction.ID,
			TransactionType:    transactionEvent.Transaction.Type,
			TransactionSampled: newBool(true),
			Exception: &model.Exception{
				Type:    "the_type",
				Message: "the_message",
				Handled: newBool(true),
				Attributes: map[string]interface{}{
					"stacktrace": "the_stacktrace",
				},
			},
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
