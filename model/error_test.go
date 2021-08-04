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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func baseException() *Exception {
	return &Exception{Message: "exception message"}
}

func (e *Exception) withCode(code string) *Exception {
	e.Code = code
	return e
}

func baseLog() *Log {
	return &Log{Message: "error log message"}
}

func TestHandleExceptionTree(t *testing.T) {
	event := &Error{
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
	}
	exceptions := flattenExceptionTree(event.Exception)

	assert.Len(t, exceptions, 7)
	for i, ex := range exceptions {
		assert.Equal(t, fmt.Sprintf("message%d", i), ex.Message)
		assert.Equal(t, fmt.Sprintf("type%d", i), ex.Type)
		assert.Nil(t, ex.Cause)
	}
	assert.Equal(t, 0, *exceptions[2].Parent)
	assert.Equal(t, 3, *exceptions[5].Parent)
	assert.Equal(t, 0, *exceptions[6].Parent)
}

func TestEventFields(t *testing.T) {
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
	log := Log{
		Level:        level,
		Message:      logMsg,
		ParamMessage: paramMsg,
		LoggerName:   loggerName,
	}

	trID := "945254c5-67a5-417e-8a4e-aa29efcbfb79"

	tests := map[string]struct {
		Error  Error
		Output common.MapStr
	}{
		"minimal": {
			Error:  Error{},
			Output: common.MapStr(nil),
		},
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
		"withFrames": {
			Error: Error{
				ID:            id,
				Culprit:       culprit,
				Exception:     &exception,
				Log:           &log,
				TransactionID: trID,
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
			fields := tc.Error.fields()
			assert.Equal(t, tc.Output, fields["error"])
		})
	}
}
