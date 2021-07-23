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
	"fmt"
	"net"
	"testing"
	"time"

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
				Timestamp:     time.Now(),
				Culprit:       culprit,
				Exception:     &exception,
				Log:           &log,
				TransactionID: trID,

				// Service name and version are required for sourcemapping.
				Metadata: Metadata{
					Service: Service{
						Name:    "myservice",
						Version: "myservice",
					},
				},
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
			output := tc.Error.toBeatEvent(context.Background())
			fields := output.Fields["error"]
			assert.Equal(t, tc.Output, fields)
		})
	}
}

func TestEvents(t *testing.T) {
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6,
		time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	exMsg := "exception message"
	trID := "945254c5-67a5-417e-8a4e-aa29efcbfb79"
	sampledTrue, sampledFalse := true, false
	transactionType := "request"

	email, userIP, userAgent := "m@m.com", "127.0.0.1", "js-1.0"
	uid := "1234567889"
	url, referer := "https://localhost", "http://localhost"
	labels := common.MapStr{"key": true}
	custom := common.MapStr{"foo.bar": "baz"}

	serviceName, agentName, version := "myservice", "go", "1.0"
	md := Metadata{
		Agent: Agent{Name: agentName, Version: version},
		Service: Service{
			Name: serviceName, Version: version,
		},
		Labels: common.MapStr{"label": 101},
	}

	mdWithContext := md
	mdWithContext.User = User{ID: uid, Email: email}
	mdWithContext.Client.IP = net.ParseIP(userIP)
	mdWithContext.UserAgent.Original = userAgent

	for name, tc := range map[string]struct {
		Error  *Error
		Output common.MapStr
		Msg    string
	}{
		"valid": {
			Error: &Error{Timestamp: timestamp, Metadata: md},
			Output: common.MapStr{
				"agent":     common.MapStr{"name": "go", "version": "1.0"},
				"service":   common.MapStr{"name": "myservice", "version": "1.0"},
				"error":     common.MapStr(nil),
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"notSampled": {
			Error: &Error{Timestamp: timestamp, Metadata: md, TransactionSampled: &sampledFalse},
			Output: common.MapStr{
				"transaction": common.MapStr{"sampled": false},
				"agent":       common.MapStr{"name": "go", "version": "1.0"},
				"service":     common.MapStr{"name": "myservice", "version": "1.0"},
				"error":       common.MapStr(nil),
				"processor":   common.MapStr{"event": "error", "name": "error"},
				"timestamp":   common.MapStr{"us": timestampUs},
				"labels":      common.MapStr{"label": 101},
			},
		},
		"withMeta": {
			Error: &Error{Timestamp: timestamp, Metadata: md, TransactionType: transactionType},
			Output: common.MapStr{
				"transaction": common.MapStr{"type": "request"},
				"error":       common.MapStr(nil),
				"processor":   common.MapStr{"event": "error", "name": "error"},
				"service":     common.MapStr{"name": "myservice", "version": "1.0"},
				"timestamp":   common.MapStr{"us": timestampUs},
				"agent":       common.MapStr{"name": "go", "version": "1.0"},
				"labels":      common.MapStr{"label": 101},
			},
		},
		"withContext": {
			Error: &Error{
				Timestamp: timestamp,
				Metadata:  mdWithContext,
				Log:       baseLog(),
				Exception: &Exception{
					Message:    exMsg,
					Stacktrace: Stacktrace{&StacktraceFrame{Filename: "myFile"}},
				},
				TransactionID:      trID,
				TransactionSampled: &sampledTrue,
				Labels:             labels,
				Page:               &Page{URL: &URL{Original: url}, Referer: referer},
				HTTP:               &HTTP{Request: &HTTPRequest{Referrer: referer}},
				URL:                &URL{Original: url},
				Custom:             custom,
			},

			Output: common.MapStr{
				"labels":     common.MapStr{"key": true, "label": 101},
				"service":    common.MapStr{"name": "myservice", "version": "1.0"},
				"agent":      common.MapStr{"name": "go", "version": "1.0"},
				"user":       common.MapStr{"id": uid, "email": email},
				"client":     common.MapStr{"ip": userIP},
				"source":     common.MapStr{"ip": userIP},
				"user_agent": common.MapStr{"original": userAgent},
				"error": common.MapStr{
					"custom": common.MapStr{
						"foo_bar": "baz",
					},
					"log": common.MapStr{"message": "error log message"},
					"exception": []common.MapStr{{
						"message": "exception message",
						"stacktrace": []common.MapStr{{
							"exclude_from_grouping": false,
							"filename":              "myFile",
						}},
					}},
					"page": common.MapStr{"url": url, "referer": referer},
				},
				"http": common.MapStr{
					"request": common.MapStr{
						"referrer": referer,
					},
				},
				"url":         common.MapStr{"original": url},
				"processor":   common.MapStr{"event": "error", "name": "error"},
				"transaction": common.MapStr{"id": "945254c5-67a5-417e-8a4e-aa29efcbfb79", "sampled": true},
				"timestamp":   common.MapStr{"us": timestampUs},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			outputEvent := tc.Error.toBeatEvent(context.Background())
			assert.Equal(t, tc.Output, outputEvent.Fields)
			assert.Equal(t, timestamp, outputEvent.Timestamp)

		})
	}
}

func TestErrorTransformPage(t *testing.T) {
	id := "123"
	urlExample := "http://example.com/path"

	tests := []struct {
		Error  Error
		Output common.MapStr
		Msg    string
	}{
		{
			Error: Error{
				ID:        id,
				Timestamp: time.Now(),
				URL:       ParseURL("https://localhost:8200/", "", ""),
				Page: &Page{
					URL: ParseURL(urlExample, "", ""),
				},
			},
			Output: common.MapStr{
				"domain":   "localhost",
				"full":     "https://localhost:8200/",
				"original": "https://localhost:8200/",
				"path":     "/",
				"port":     8200,
				"scheme":   "https",
			},
			Msg: "With Page URL and Request URL",
		},
	}

	for idx, test := range tests {
		output := test.Error.toBeatEvent(context.Background())
		assert.Equal(t, test.Output, output.Fields["url"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
