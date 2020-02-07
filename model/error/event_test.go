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

package error

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/utility"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/sourcemap/test"

	"github.com/elastic/beats/libbeat/common"
)

func baseException() *Exception {
	msg := "exception message"
	return &Exception{Message: &msg}
}

func (e *Exception) withCode(code interface{}) *Exception {
	e.Code = code
	return e
}

func (e *Exception) withType(etype string) *Exception {
	e.Type = &etype
	return e
}

func (e *Exception) withFrames(frames []*m.StacktraceFrame) *Exception {
	e.Stacktrace = frames
	return e
}

func baseLog() *Log {
	return &Log{Message: "error log message"}
}

func (l *Log) withParamMsg(msg string) *Log {
	l.ParamMessage = &msg
	return l
}

func (l *Log) withFrames(frames []*m.StacktraceFrame) *Log {
	l.Stacktrace = frames
	return l
}

func TestErrorEventDecode(t *testing.T) {
	timestamp := json.Number("1496170407154000")
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)

	id, culprit, lineno := "123", "foo()", 2
	exceptionMsg := "Exception Msg"
	parentId, traceId, transactionId := "0123456789abcdef", "01234567890123456789abcdefabcdef", "abcdefabcdef0000"
	name, userId, email, userIp := "jane", "abc123", "j@d.com", "127.0.0.1"
	pUrl, referer, origUrl := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	code, module, attrs, exType, handled := "200", "a", map[string]interface{}{}, "errorEx", false
	exMsg, paramMsg, level, logger := "Exception Msg", "log pm", "error", "mylogger"
	transactionSampled := true
	transactionType := "request"
	labels := m.Labels{"ab": "c"}
	ua := "go-1.1"
	user := metadata.User{Name: &name, Email: &email, IP: net.ParseIP(userIp), Id: &userId, UserAgent: &ua}
	page := m.Page{Url: &pUrl, Referer: &referer}
	custom := m.Custom{"a": "b"}
	request := m.Req{Method: "post", Socket: &m.Socket{}, Headers: http.Header{"User-Agent": []string{ua}}, Cookies: map[string]interface{}{"a": "b"}}
	response := m.Resp{Finished: new(bool), Headers: http.Header{"Content-Type": []string{"text/html"}}}
	h := m.Http{Request: &request, Response: &response}
	ctxUrl := m.Url{Original: &origUrl}

	for name, test := range map[string]struct {
		input        interface{}
		experimental bool
		error        string
		event        *Event
	}{
		"invalid type": {input: "", event: nil},
		"error decoding timestamp": {
			input: map[string]interface{}{
				"timestamp": 123,
				"id":        id,
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
			},
			error: utility.ErrFetch.Error(),
		},
		"error decoding transaction id": {
			input: map[string]interface{}{
				"transaction_id": 123,
				"id":             id,
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
			},
			error: "expected string or null, but got number",
		},
		"only parent id given": {
			input: map[string]interface{}{
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"id": id, "culprit": culprit, "context": map[string]interface{}{}, "timestamp": timestamp,
				"parent_id": "123"},
			error: "missing properties: \"trace_id\"",
		},
		"only trace id given": {
			input: map[string]interface{}{
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"id": id, "culprit": culprit, "context": map[string]interface{}{}, "timestamp": timestamp,
				"trace_id": "123"},
			error: "missing properties: \"parent_id\"",
		},
		"invalid type for exception stacktrace": {
			input: map[string]interface{}{
				"timestamp": timestamp,
				"exception": map[string]interface{}{
					"message":    "Exception Msg",
					"stacktrace": "123",
				},
				"id": "id1",
			},
			error: "expected array or null, but got string",
		},
		"invalid type for log stacktrace": {
			input: map[string]interface{}{
				"timestamp": timestamp,
				"log": map[string]interface{}{
					"message":    "Log Msg",
					"stacktrace": "123",
				},
				"id": "id1",
			},
			error: "expected array or null, but got string",
		},
		"minimal valid error": {
			input: map[string]interface{}{
				"id":      id,
				"culprit": culprit,
				"context": map[string]interface{}{},
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"timestamp": timestamp},
			event: &Event{
				Id:      &id,
				Culprit: &culprit,
				Exception: &Exception{
					Message:    &exceptionMsg,
					Stacktrace: m.Stacktrace{},
				},
				Timestamp: timestampParsed,
			},
		},
		"valid error experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "timestamp": timestamp,
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"context": map[string]interface{}{"foo": []string{"a", "b"}}},
			event: &Event{
				Id:        &id,
				Culprit:   &culprit,
				Timestamp: timestampParsed,
				Exception: &Exception{
					Message:    &exceptionMsg,
					Stacktrace: m.Stacktrace{},
				},
			},
			experimental: true,
		},
		"valid error experimental=false": {
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "timestamp": timestamp,
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"context": map[string]interface{}{"experimental": []string{"a", "b"}}},
			event: &Event{
				Id:        &id,
				Culprit:   &culprit,
				Timestamp: timestampParsed,
				Exception: &Exception{
					Message:    &exceptionMsg,
					Stacktrace: m.Stacktrace{},
				},
			},
		},
		"valid error experimental=true": {
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "timestamp": timestamp,
				"exception": map[string]interface{}{
					"message": "Exception Msg",
				},
				"context": map[string]interface{}{"experimental": []string{"a", "b"}}},
			event: &Event{
				Id:           &id,
				Culprit:      &culprit,
				Timestamp:    timestampParsed,
				Experimental: []string{"a", "b"},
				Exception: &Exception{
					Message:    &exceptionMsg,
					Stacktrace: m.Stacktrace{},
				},
			},
			experimental: true,
		},
		"full valid error event": {
			input: map[string]interface{}{
				"timestamp": timestamp,
				"id":        id,
				"context": map[string]interface{}{
					"a":      "b",
					"user":   map[string]interface{}{"username": name, "email": email, "ip": userIp, "id": userId},
					"tags":   map[string]interface{}{"ab": "c"},
					"page":   map[string]interface{}{"url": pUrl, "referer": referer},
					"custom": map[string]interface{}{"a": "b"},
					"request": map[string]interface{}{
						"method":  "POST",
						"url":     map[string]interface{}{"raw": "127.0.0.1"},
						"headers": map[string]interface{}{"user-agent": ua},
						"cookies": map[string]interface{}{"a": "b"}},
					"response": map[string]interface{}{
						"finished": false,
						"headers":  map[string]interface{}{"Content-Type": "text/html"}},
				},
				"exception": map[string]interface{}{
					"message": "Exception Msg",
					"code":    code, "module": module, "attributes": attrs,
					"type": exType, "handled": handled,
					"stacktrace": []interface{}{
						map[string]interface{}{
							"filename": "file",
						},
					},
				},
				"log": map[string]interface{}{
					"message":       "Log Msg",
					"param_message": paramMsg,
					"level":         level, "logger_name": logger,
					"stacktrace": []interface{}{
						map[string]interface{}{
							"filename": "log file", "lineno": 2.0,
						},
					},
				},
				"transaction_id": transactionId,
				"parent_id":      parentId,
				"trace_id":       traceId,
				"transaction":    map[string]interface{}{"sampled": transactionSampled, "type": transactionType},
			},
			event: &Event{
				Timestamp: timestampParsed,
				Id:        &id,
				User:      &user,
				Labels:    &labels,
				Page:      &page,
				Custom:    &custom,
				Http:      &h,
				Url:       &ctxUrl,
				Client:    &m.Client{IP: net.ParseIP(userIp)},
				Exception: &Exception{
					Message:    &exMsg,
					Code:       code,
					Type:       &exType,
					Module:     &module,
					Attributes: attrs,
					Handled:    &handled,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: pointer("file")},
					},
				},
				Log: &Log{
					Message:      "Log Msg",
					ParamMessage: &paramMsg,
					Level:        &level,
					LoggerName:   &logger,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: pointer("log file"), Lineno: &lineno},
					},
				},
				TransactionId:      &transactionId,
				TransactionSampled: &transactionSampled,
				TransactionType:    &transactionType,
				ParentId:           &parentId,
				TraceId:            &traceId,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			event, err := Decode(test.input, time.Now(), metadata.Metadata{}, test.experimental)
			if test.event != nil {
				if err != nil {
					assert.Equal(t, err, err.Error())
				}
				require.NotNil(t, event)
				assert.Equal(t, test.event, event)
			} else {
				assert.Contains(t, err.Error(), test.error)
			}
		})
	}
}

func TestHandleExceptionTree(t *testing.T) {
	errorEvent := map[string]interface{}{
		"id": "id",
		"exception": map[string]interface{}{
			"message": "message0",
			"type":    "type0",
			"stacktrace": []interface{}{
				map[string]interface{}{
					"filename": "file0",
				},
			},
			"cause": []interface{}{
				map[string]interface{}{"message": "message1", "type": "type1"},
				map[string]interface{}{"message": "message2", "type": "type2", "cause": []interface{}{
					map[string]interface{}{"message": "message3", "type": "type3", "cause": []interface{}{
						map[string]interface{}{"message": "message4", "type": "type4"},
						map[string]interface{}{"message": "message5", "type": "type5"},
					}},
				}},
				map[string]interface{}{"message": "message6", "type": "type6"},
			},
		},
	}
	event, err := Decode(errorEvent, time.Now(), metadata.Metadata{}, false)
	require.NoError(t, err)

	exceptions := flattenExceptionTree(event.Exception)

	assert.Len(t, exceptions, 7)
	for i, ex := range exceptions {
		assert.Equal(t, fmt.Sprintf("message%d", i), *ex.Message)
		assert.Equal(t, fmt.Sprintf("type%d", i), *ex.Type)
		assert.Nil(t, ex.Cause)
	}
	assert.Equal(t, 0, *exceptions[2].Parent)
	assert.Equal(t, 3, *exceptions[5].Parent)
	assert.Equal(t, 0, *exceptions[6].Parent)
}

func TestDecodingAnomalies(t *testing.T) {

	t.Run("exception decoder doesn't erase existing errors", func(t *testing.T) {
		badID := map[string]interface{}{
			"id": 7.4,
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
			},
		}
		event, err := Decode(badID, time.Now(), metadata.Metadata{}, false)
		assert.Error(t, err)
		assert.Nil(t, event)
	})

	t.Run("exception decoding error bubbles up", func(t *testing.T) {
		badException := map[string]interface{}{
			"id": "id",
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
				"cause": []interface{}{
					map[string]interface{}{"message": "message1", "type": 7.4},
				},
			},
		}
		event, err := Decode(badException, time.Now(), metadata.Metadata{}, false)
		assert.Error(t, err)
		assert.Nil(t, event)
	})

	t.Run("wrong cause type", func(t *testing.T) {
		badException := map[string]interface{}{
			"id": "id",
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
				"cause":   []interface{}{74},
			},
		}
		event, err := Decode(badException, time.Now(), metadata.Metadata{}, false)
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "error validating JSON document against schema"), err.Error())
		assert.Nil(t, event)
	})

	t.Run("handle nil exceptions", func(t *testing.T) {
		input := map[string]interface{}{
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
				"cause": []interface{}{
					map[string]interface{}{"message": "message1", "type": "type1", "id": "id2", "cause": []interface{}{}},
					map[string]interface{}{},
				},
			},
			"id":        "id",
			"timestamp": json.Number("1496170407154000"),
		}

		event, err := Decode(input, time.Now(), metadata.Metadata{}, false)
		require.NoError(t, err)
		assert.NotNil(t, event)

		assert.Equal(t, *event.Exception.Message, "message0")
		assert.Len(t, event.Exception.Cause, 1)
		assert.Equal(t, *event.Exception.Cause[0].Message, "message1")
		assert.Nil(t, event.Exception.Cause[0].Cause)
	})
}

func TestEventFields(t *testing.T) {
	id := "45678"
	culprit := "some trigger"

	errorType := "error type"
	codeFloat := 13.0
	module := "error module"
	exMsg := "exception message"
	handled := false
	attributes := common.MapStr{"k1": "val1"}
	exception := Exception{
		Type:       &errorType,
		Code:       codeFloat,
		Message:    &exMsg,
		Module:     &module,
		Handled:    &handled,
		Attributes: attributes,
		Stacktrace: []*m.StacktraceFrame{{Filename: pointer("st file")}},
	}

	level := "level"
	loggerName := "logger"
	logMsg := "error log message"
	paramMsg := "param message"
	log := Log{
		Level:        &level,
		Message:      logMsg,
		ParamMessage: &paramMsg,
		LoggerName:   &loggerName,
	}
	baseExceptionHash := md5.New()
	io.WriteString(baseExceptionHash, *baseException().Message)
	// 706a38d554b47b8f82c6b542725c05dc
	baseExceptionGroupingKey := hex.EncodeToString(baseExceptionHash.Sum(nil))

	baseLogHash := md5.New()
	io.WriteString(baseLogHash, baseLog().Message)
	baseLogGroupingKey := hex.EncodeToString(baseLogHash.Sum(nil))
	trId := "945254c5-67a5-417e-8a4e-aa29efcbfb79"

	tests := map[string]struct {
		Event  Event
		Output common.MapStr
	}{
		"minimal": {
			Event: Event{},
			Output: common.MapStr{
				"grouping_key": hex.EncodeToString(md5.New().Sum(nil)),
			},
		},
		"withLog": {
			Event: Event{Log: baseLog()},
			Output: common.MapStr{
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseLogGroupingKey,
			},
		},
		"withLogAndException": {
			Event: Event{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"withException": {
			Event: Event{Exception: baseException()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"stringCode": {
			Event: Event{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"intCode": {
			Event: Event{Exception: baseException().withCode(13)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"floatCode": {
			Event: Event{Exception: baseException().withCode(13.0)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"withFrames": {
			Event: Event{
				Id:            &id,
				Timestamp:     time.Now(),
				Culprit:       &culprit,
				Exception:     &exception,
				Log:           &log,
				TransactionId: &trId,
			},
			Output: common.MapStr{
				"id":      "45678",
				"culprit": "some trigger",
				"exception": []common.MapStr{{
					"stacktrace": []common.MapStr{{
						"filename":              "st file",
						"exclude_from_grouping": false,
						"sourcemap": common.MapStr{
							"error":   "Colno mandatory for sourcemapping.",
							"updated": false,
						},
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
				"grouping_key": "2725d2590215a6e975f393bf438f90ef",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := "myservice"
			tc.Event.Metadata = metadata.Metadata{Service: &metadata.Service{Name: &s, Version: &s}}
			output := tc.Event.Transform(nil, nil, &sourcemap.Store{})
			require.Len(t, output, 1)
			fields := output[0].Fields["error"]
			assert.Equal(t, tc.Output, fields)
		})
	}
}

func TestEvents(t *testing.T) {
	timestamp := time.Date(2019, 1, 3, 15, 17, 4, 908.596*1e6,
		time.FixedZone("+0100", 3600))
	timestampUs := timestamp.UnixNano() / 1000
	serviceName, agentName, version := "myservice", "go", "1.0"
	service := metadata.Service{
		Name: &serviceName, Version: &version, Agent: metadata.Agent{Name: &agentName, Version: &version},
	}
	exMsg := "exception message"
	trId := "945254c5-67a5-417e-8a4e-aa29efcbfb79"
	sampledTrue, sampledFalse := true, false
	transactionType := "request"

	email, userIp, userAgent := "m@m.com", "127.0.0.1", "js-1.0"
	uid := "1234567889"
	url, referer := "https://localhost", "http://localhost"
	labels := m.Labels(common.MapStr{"key": true})
	custom := m.Custom(common.MapStr{"foo": "bar"})
	metadataLabels := common.MapStr{"label": 101}
	for name, tc := range map[string]struct {
		Transformable *Event
		Output        common.MapStr
		Msg           string
	}{
		"valid": {
			Transformable: &Event{Timestamp: timestamp},
			Output: common.MapStr{
				"agent":   common.MapStr{"name": "go", "version": "1.0"},
				"service": common.MapStr{"name": "myservice", "version": "1.0"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"user":      common.MapStr{"id": uid},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"notSampled": {
			Transformable: &Event{Timestamp: timestamp, TransactionSampled: &sampledFalse},
			Output: common.MapStr{
				"transaction": common.MapStr{"sampled": false},
				"agent":       common.MapStr{"name": "go", "version": "1.0"},
				"service":     common.MapStr{"name": "myservice", "version": "1.0"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"user":      common.MapStr{"id": uid},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"withMeta": {
			Transformable: &Event{Timestamp: timestamp, TransactionType: &transactionType},
			Output: common.MapStr{
				"transaction": common.MapStr{"type": "request"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"service":   common.MapStr{"name": "myservice", "version": "1.0"},
				"user":      common.MapStr{"id": uid},
				"timestamp": common.MapStr{"us": timestampUs},
				"agent":     common.MapStr{"name": "go", "version": "1.0"},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"withContext": {
			Transformable: &Event{
				Timestamp: timestamp,
				Log:       baseLog(),
				Exception: &Exception{
					Message:    &exMsg,
					Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: pointer("myFile")}},
				},
				TransactionId:      &trId,
				TransactionSampled: &sampledTrue,
				User:               &metadata.User{Email: &email, IP: net.ParseIP(userIp), UserAgent: &userAgent},
				Labels:             &labels,
				Page:               &m.Page{Url: &url, Referer: &referer},
				Custom:             &custom,
				Client:             &m.Client{IP: net.ParseIP("192.0.14.10")},
			},

			Output: common.MapStr{
				"labels":     common.MapStr{"key": true, "label": 101},
				"service":    common.MapStr{"name": "myservice", "version": "1.0"},
				"agent":      common.MapStr{"name": "go", "version": "1.0"},
				"user":       common.MapStr{"email": email},
				"client":     common.MapStr{"ip": "192.0.14.10"},
				"source":     common.MapStr{"ip": "192.0.14.10"},
				"user_agent": common.MapStr{"original": userAgent},
				"error": common.MapStr{
					"custom": common.MapStr{
						"foo": "bar",
					},
					"grouping_key": "a61a65e048f403d9bcb2863d517fb48d",
					"log":          common.MapStr{"message": "error log message"},
					"exception": []common.MapStr{{
						"message": "exception message",
						"stacktrace": []common.MapStr{{
							"exclude_from_grouping": false,
							"filename":              "myFile",
							"sourcemap": common.MapStr{
								"error":   "Colno mandatory for sourcemapping.",
								"updated": false,
							},
						}},
					}},
					"page": common.MapStr{"url": url, "referer": referer},
				},
				"processor":   common.MapStr{"event": "error", "name": "error"},
				"transaction": common.MapStr{"id": "945254c5-67a5-417e-8a4e-aa29efcbfb79", "sampled": true},
				"timestamp":   common.MapStr{"us": timestampUs},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.Transformable.Metadata = *metadata.NewMetadata(&service, nil, nil, &metadata.User{Id: &uid}, metadataLabels)
			outputEvents := tc.Transformable.Transform(nil, nil, &sourcemap.Store{})
			require.Len(t, outputEvents, 1)
			outputEvent := outputEvents[0]
			assert.Equal(t, tc.Output, outputEvent.Fields)
			assert.Equal(t, timestamp, outputEvent.Timestamp)

		})
	}
}

func TestCulprit(t *testing.T) {
	c := "foo"
	fct := "fct"
	truthy := true
	st := m.Stacktrace{
		&m.StacktraceFrame{Filename: pointer("a"), Function: &fct, Sourcemap: m.Sourcemap{}},
	}
	stUpdate := m.Stacktrace{
		&m.StacktraceFrame{Filename: pointer("a"), Function: &fct, Sourcemap: m.Sourcemap{}},
		&m.StacktraceFrame{Filename: pointer("a"), LibraryFrame: &truthy, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: pointer("f"), Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: pointer("bar"), Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
	}
	tests := []struct {
		event   Event
		culprit string
		msg     string
	}{
		{
			event:   Event{Culprit: &c},
			culprit: "foo",
			msg:     "No Sourcemap in config",
		},
		{
			event:   Event{Culprit: &c},
			culprit: "foo",
			msg:     "No Stacktrace Frame given.",
		},
		{
			event:   Event{Culprit: &c, Log: &Log{Stacktrace: st}},
			culprit: "foo",
			msg:     "Log.StacktraceFrame has no updated frame",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  pointer("f"),
							Classname: pointer("xyz"),
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
			},
			culprit: "f",
			msg:     "Adapt culprit to first valid Log.StacktraceFrame filename information.",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Classname: pointer("xyz"),
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
			},
			culprit: "xyz",
			msg:     "Adapt culprit Log.StacktraceFrame classname information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Exception: &Exception{Stacktrace: stUpdate},
			},
			culprit: "f in fct",
			msg:     "Adapt culprit to first valid Exception.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Log:       &Log{Stacktrace: st},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			culprit: "f in fct",
			msg:     "Log and Exception StacktraceFrame given, only one changes culprit.",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  pointer("a"),
							Function:  &fct,
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			culprit: "a in fct",
			msg:     "Log Stacktrace is prioritized over Exception StacktraceFrame",
		},
	}
	for idx, test := range tests {
		t.Run(fmt.Sprint(idx), func(t *testing.T) {
			test.event.updateCulprit()
			assert.Equal(t, test.culprit, *test.event.Culprit,
				fmt.Sprintf("(%v) %s: expected <%v>, received <%v>", idx, test.msg, test.culprit, *test.event.Culprit))
		})
	}
}

func TestEmptyGroupingKey(t *testing.T) {
	emptyGroupingKey := hex.EncodeToString(md5.New().Sum(nil))
	e := Event{}
	assert.Equal(t, emptyGroupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestExplicitGroupingKey(t *testing.T) {
	attr := "hello world"
	diffAttr := "huhu"

	groupingKey := hex.EncodeToString(md5With(attr))

	e1 := Event{Log: baseLog().withParamMsg(attr)}
	e2 := Event{Exception: baseException().withType(attr)}
	e3 := Event{Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &attr}})}
	e4 := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &attr}})}
	e5 := Event{
		Log:       baseLog().withFrames([]*m.StacktraceFrame{{Function: &diffAttr}}),
		Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &attr}}),
	}

	for idx, e := range []Event{e1, e2, e3, e4, e5} {
		assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)), "grouping_key mismatch", idx)
	}
}

func TestFramesUsableForGroupingKey(t *testing.T) {
	webpackLineno := 77
	tmpLineno := 45
	st1 := m.Stacktrace{
		&m.StacktraceFrame{Filename: pointer("/a/b/c"), ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: pointer("webpack"), Lineno: &webpackLineno, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: pointer("~/tmp"), Lineno: &tmpLineno, ExcludeFromGrouping: true},
	}
	st2 := m.Stacktrace{
		&m.StacktraceFrame{Filename: pointer("/a/b/c"), ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: pointer("webpack"), Lineno: &webpackLineno, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: pointer("~/tmp"), Lineno: &tmpLineno, ExcludeFromGrouping: false},
	}
	exMsg := "base exception"
	e1 := Event{Exception: &Exception{Message: &exMsg, Stacktrace: st1}}
	e2 := Event{Exception: &Exception{Message: &exMsg, Stacktrace: st2}}
	key1 := e1.calcGroupingKey(flattenExceptionTree(e1.Exception))
	key2 := e2.calcGroupingKey(flattenExceptionTree(e2.Exception))
	assert.NotEqual(t, key1, key2)
}

func TestFallbackGroupingKey(t *testing.T) {
	lineno := 12
	filename := "file"

	groupingKey := hex.EncodeToString(md5With(filename))

	e := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))

	e = Event{Exception: baseException(), Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &lineno, Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestNoFallbackGroupingKey(t *testing.T) {
	lineno := 1
	function := "function"
	filename := "file"
	module := "module"

	groupingKey := hex.EncodeToString(md5With(module, function))

	e := Event{
		Exception: baseException().withFrames([]*m.StacktraceFrame{
			{Lineno: &lineno, Module: &module, Filename: &filename, Function: &function},
		}),
	}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestGroupableEvents(t *testing.T) {
	value := "value"
	name := "name"
	var tests = []struct {
		e1     Event
		e2     Event
		result bool
	}{
		{
			e1: Event{
				Log: baseLog().withParamMsg(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Event{
				Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{}),
			},
			e2:     Event{},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &value}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &name}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: &name}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: &name}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: pointer("nameEx")}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &name}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Filename: &name}}),
			},
			result: true,
		},
	}

	for idx, test := range tests {
		sameGroup := test.e1.calcGroupingKey(flattenExceptionTree(test.e1.Exception)) ==
			test.e2.calcGroupingKey(flattenExceptionTree(test.e2.Exception))
		assert.Equal(t, test.result, sameGroup,
			"grouping_key mismatch", idx)
	}
}

func md5With(args ...string) []byte {
	md5 := md5.New()
	for _, arg := range args {
		md5.Write([]byte(arg))
	}
	return md5.Sum(nil)
}

func TestSourcemapping(t *testing.T) {
	str := "foo"
	col, line, path := 23, 1, "../a/b"
	exMsg := "exception message"
	event1 := Event{Exception: &Exception{Message: &exMsg,
		Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: pointer("/a/b/c"), Lineno: &line, Colno: &col, AbsPath: &path}},
	}, Metadata: metadata.Metadata{Service: &metadata.Service{Name: &str, Version: &str}}}
	event2 := Event{Exception: &Exception{Message: &exMsg,
		Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: pointer("/a/b/c"), Lineno: &line, Colno: &col, AbsPath: &path}},
	}, Metadata: metadata.Metadata{Service: &metadata.Service{Name: &str, Version: &str}}}

	// transform without sourcemap store
	transformedNoSourcemap := event1.fields(nil, nil, nil)

	// transform with sourcemap store
	store, err := sourcemap.NewStore(test.ESClientWithValidSourcemap(t), "apm-*sourcemap*", time.Minute)
	require.NoError(t, err)
	transformedWithSourcemap := event2.fields(nil, nil, store)

	// ensure events have different line number and grouping keys
	assert.Equal(t, 1, *event1.Exception.Stacktrace[0].Lineno)
	assert.Equal(t, 5, *event2.Exception.Stacktrace[0].Lineno)
	assert.NotEqual(t, transformedNoSourcemap["grouping_key"], transformedWithSourcemap["grouping_key"])
}

func pointer(s string) *string {
	return &s
}
