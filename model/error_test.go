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

package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func baseException() *Exception {
	return &Exception{Message: "exception message"}
}

func (e *Exception) withCode(code string) *Exception {
	e.Code = code
	return e
}

func baseLog() *ErrorLog {
	return &ErrorLog{Message: "error log message"}
}

func TestHandleExceptionTree(t *testing.T) {
	event := APMEvent{
		Error: &Error{
			ID: "id",
			Exception: &Exception{
				Message: "message0",
				Type:    "type0",
				Stacktrace: Stacktrace{{
					Filename: "file0",
				}},
				Cause: []Exception{{
					Message: "message1",
					Type:    "type1",
				}, {
					Message: "message2",
					Type:    "type2",
					Cause: []Exception{{
						Message: "message3",
						Type:    "type3",
						Cause: []Exception{{
							Message: "message4",
							Type:    "type4",
						}, {
							Message: "message5",
							Type:    "type5",
						}},
					}},
				}, {
					Message: "message6",
					Type:    "type6",
				}},
			},
		},
	}

	beatEvent := event.BeatEvent(context.Background())
	exceptionField, err := beatEvent.Fields.GetValue("error.exception")
	require.NoError(t, err)
	assert.Equal(t, []common.MapStr{{
		"message": "message0",
		"stacktrace": []common.MapStr{{
			"exclude_from_grouping": false,
			"filename":              "file0",
		}},
		"type": "type0",
	}, {
		"message": "message1",
		"type":    "type1",
	}, {
		"message": "message2",
		"type":    "type2",
		"parent":  0,
	}, {
		"message": "message3",
		"type":    "type3",
	}, {
		"message": "message4",
		"type":    "type4",
	}, {
		"message": "message5",
		"type":    "type5",
		"parent":  3,
	}, {
		"message": "message6",
		"type":    "type6",
		"parent":  0,
	}}, exceptionField)
}

func TestErrorFieldsEmpty(t *testing.T) {
	event := APMEvent{Error: &Error{}}
	beatEvent := event.BeatEvent(context.Background())
	assert.Empty(t, beatEvent.Fields)
}

func TestErrorFields(t *testing.T) {
	id := "45678"
	culprit := "some trigger"

	errorType := "error type"
	module := "error module"
	exMsg := "exception message"
	handled := false
	attributes := common.MapStr{"k1": "val1"}
	exception := Exception{
		Type:       errorType,
		Code:       "13",
		Message:    exMsg,
		Module:     module,
		Handled:    &handled,
		Attributes: attributes,
		Stacktrace: []*StacktraceFrame{{Filename: "st file"}},
	}

	level := "level"
	loggerName := "logger"
	logMsg := "error log message"
	paramMsg := "param message"
	log := ErrorLog{
		Level:        level,
		Message:      logMsg,
		ParamMessage: paramMsg,
		LoggerName:   loggerName,
	}

	tests := map[string]struct {
		Error  Error
		Output common.MapStr
	}{
		"withGroupingKey": {
			Error:  Error{GroupingKey: "foo"},
			Output: common.MapStr{"grouping_key": "foo"},
		},
		"withLog": {
			Error: Error{Log: baseLog()},
			Output: common.MapStr{
				"log": common.MapStr{"message": "error log message"},
			},
		},
		"withLogAndException": {
			Error: Error{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception": []common.MapStr{{"message": "exception message"}},
				"log":       common.MapStr{"message": "error log message"},
			},
		},
		"withException": {
			Error: Error{Exception: baseException()},
			Output: common.MapStr{
				"exception": []common.MapStr{{"message": "exception message"}},
			},
		},
		"stringCode": {
			Error: Error{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception": []common.MapStr{{"message": "exception message", "code": "13"}},
			},
		},
		"withStackTrace": {
			Error: Error{StackTrace: "raw stack trace"},
			Output: common.MapStr{
				"stack_trace": "raw stack trace",
			},
		},
		"withFrames": {
			Error: Error{
				ID:        id,
				Culprit:   culprit,
				Exception: &exception,
				Log:       &log,
			},
			Output: common.MapStr{
				"id":      "45678",
				"culprit": "some trigger",
				"exception": []common.MapStr{{
					"stacktrace": []common.MapStr{{
						"filename":              "st file",
						"exclude_from_grouping": false,
					}},
					"code":       "13",
					"message":    "exception message",
					"module":     "error module",
					"attributes": common.MapStr{"k1": "val1"},
					"type":       "error type",
					"handled":    false,
				}},
				"log": common.MapStr{
					"message":       "error log message",
					"param_message": "param message",
					"logger_name":   "logger",
					"level":         "level",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event := APMEvent{Error: &tc.Error}
			beatEvent := event.BeatEvent(context.Background())
			assert.Equal(t, tc.Output, beatEvent.Fields["error"])
		})
	}
}
