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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
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
	e.Stacktrace = m.Stacktrace(frames)
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
	l.Stacktrace = m.Stacktrace(frames)
	return l
}

func TestErrorEventDecode(t *testing.T) {
	timestamp := json.Number("1496170407154000")
	timestampParsed, _ := time.Parse(time.RFC3339, "2017-05-30T18:53:27.154Z")

	id, culprit := "123", "foo()"
	parentId, traceId, transactionId := "0123456789abcdef", "01234567890123456789abcdefabcdef", "abcdefabcdef0000"
	name, userId, email, userIp := "jane", "abc123", "j@d.com", "127.0.0.1"
	pUrl, referer, origUrl := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	code, module, attrs, exType, handled := "200", "a", "attr", "errorEx", false
	exMsg, paramMsg, level, logger := "Exception Msg", "log pm", "error", "mylogger"
	transactionSampled := true
	transactionType := "request"
	label := m.Label{"ab": "c"}
	user := metadata.User{Name: &name, Email: &email, IP: &userIp, Id: &userId}
	page := m.Page{Url: &pUrl, Referer: &referer}
	custom := m.Custom{"a": "b"}
	request := m.Req{Method: "post", Socket: &m.Socket{}, Headers: &m.Headers{"user-agent": "go-1.1"}, Cookies: map[string]interface{}{"a": "b"}}
	response := m.Resp{Finished: new(bool), Headers: &m.Headers{"Content-Type": "text/html"}}
	http := m.Http{Request: &request, Response: &response}
	ctxUrl := m.Url{Original: &origUrl}
	context := m.Context{User: &user, Label: &label, Page: &page, Http: &http, Url: &ctxUrl, Custom: &custom}

	for idx, test := range []struct {
		input       interface{}
		err, inpErr error
		e           *Event
	}{
		{input: nil, err: errors.New("Input missing for decoding Event"), e: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), e: nil},
		{input: "", err: errors.New("Invalid type for error event"), e: nil},
		{
			input: map[string]interface{}{"timestamp": 123},
			err:   errors.New("Error fetching field"),
			e:     nil,
		},
		{
			input: map[string]interface{}{"transaction_id": 123},
			err:   errors.New("Error fetching field"),
			e:     nil,
		},
		{
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "context": map[string]interface{}{}, "timestamp": timestamp},
			err: nil,
			e: &Event{
				Id:        &id,
				Culprit:   &culprit,
				Context:   &m.Context{},
				Timestamp: timestampParsed,
			},
		},
		{
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "context": map[string]interface{}{}, "timestamp": timestamp,
				"parent_id": 123},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			input: map[string]interface{}{
				"id": id, "culprit": culprit, "context": map[string]interface{}{}, "timestamp": timestamp,
				"trace_id": 123},
			err: errors.New("Error fetching field"),
			e:   nil,
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"exception": map[string]interface{}{},
				"log":       map[string]interface{}{},
			},
			err: nil,
			e: &Event{
				Timestamp: timestampParsed,
				Context:   &m.Context{},
			},
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"exception": map[string]interface{}{
					"message":    "Exception Msg",
					"stacktrace": "123",
				},
			},
			err: errors.New("Invalid type for stacktrace"),
			e:   nil,
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"log": map[string]interface{}{
					"message":    "Log Msg",
					"stacktrace": "123",
				},
			},
			err: errors.New("Invalid type for stacktrace"),
			e:   nil,
		},
		{
			input: map[string]interface{}{
				"timestamp": timestamp,
				"context": map[string]interface{}{
					"a":      "b",
					"user":   map[string]interface{}{"username": name, "email": email, "ip": userIp, "id": userId},
					"tags":   map[string]interface{}{"ab": "c"},
					"page":   map[string]interface{}{"url": pUrl, "referer": referer},
					"custom": map[string]interface{}{"a": "b"},
					"request": map[string]interface{}{
						"method":  "POST",
						"url":     map[string]interface{}{"raw": "127.0.0.1"},
						"headers": map[string]interface{}{"user-agent": "go-1.1"},
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
							"filename": "file", "lineno": 1.0,
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
			err: nil,
			e: &Event{
				Timestamp: timestampParsed,
				User:      &user,
				Label:     &label,
				Page:      &page,
				Custom:    &custom,
				Http:      &http,
				Url:       &ctxUrl,
				Context:   &context,
				Exception: &Exception{
					Message:    &exMsg,
					Code:       code,
					Type:       &exType,
					Module:     &module,
					Attributes: attrs,
					Handled:    &handled,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: "file", Lineno: 1},
					},
				},
				Log: &Log{
					Message:      "Log Msg",
					ParamMessage: &paramMsg,
					Level:        &level,
					LoggerName:   &logger,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: "log file", Lineno: 2},
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
		transformable, err := DecodeEvent(test.input, test.inpErr)
		if test.e != nil && assert.NotNil(t, transformable) {
			event := transformable.(*Event)
			assert.Equal(t, test.e, event, fmt.Sprintf("Failed at idx %v", idx))
		}

		assert.Equal(t, test.err, err, fmt.Sprintf("Failed at idx %v", idx))
	}
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
		Stacktrace: []*m.StacktraceFrame{{Filename: "st file"}},
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

	user := metadata.User{Id: &id}
	context := m.Context{User: &user}

	baseExceptionHash := md5.New()
	io.WriteString(baseExceptionHash, *baseException().Message)
	// 706a38d554b47b8f82c6b542725c05dc
	baseExceptionGroupingKey := hex.EncodeToString(baseExceptionHash.Sum(nil))

	baseLogHash := md5.New()
	io.WriteString(baseLogHash, baseLog().Message)
	baseLogGroupingKey := hex.EncodeToString(baseLogHash.Sum(nil))
	trId := "945254c5-67a5-417e-8a4e-aa29efcbfb79"

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"grouping_key": hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event",
		},
		{
			Event: Event{Log: baseLog()},
			Output: common.MapStr{
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseLogGroupingKey,
			},
			Msg: "Minimal Event wth log",
		},
		{
			Event: Event{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth log and exception",
		},
		{
			Event: Event{Exception: baseException()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception",
		},
		{
			Event: Event{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception and string code",
		},
		{
			Event: Event{Exception: baseException().withCode(13)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth exception and int code",
		},
		{
			Event: Event{Exception: baseException().withCode(13.0)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth exception and float code",
		},
		{
			Event: Event{
				Id:            &id,
				Timestamp:     time.Now(),
				Culprit:       &culprit,
				Context:       &context,
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
						"line":                  common.MapStr{"number": 0},
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
				"grouping_key": "d47ca09e1cfd512804f5d55cecd34262",
			},
			Msg: "Event with frames",
		},
	}

	tctx := &transform.Context{
		Config: transform.Config{SmapMapper: &sourcemap.SmapMapper{}},
		Metadata: metadata.Metadata{
			Service: &metadata.Service{Name: "myService"},
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform(tctx)
		require.Len(t, output, 1)
		fields := output[0].Fields["error"]
		assert.Equal(t, test.Output, fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEvents(t *testing.T) {

	timestamp, _ := time.Parse(time.RFC3339, "2019-01-03T15:17:04.908596+01:00")
	timestampUs := timestamp.UnixNano() / 1000
	service := metadata.Service{
		Name: "myservice", Agent: metadata.Agent{Name: "go", Version: "1.0"},
	}
	exMsg := "exception message"
	trId := "945254c5-67a5-417e-8a4e-aa29efcbfb79"
	sampledTrue, sampledFalse := true, false
	transactionType := "request"

	email, userIp, userAgent := "m@m.com", "127.0.0.1", "js-1.0"
	uid := "1234567889"
	url, referer := "https://localhost", "http://localhost"
	label := m.Label(common.MapStr{"key": true})
	user := metadata.User{Email: &email, IP: &userIp}
	page := m.Page{Url: &url, Referer: &referer}
	custom := m.Custom(common.MapStr{"foo": "bar"})
	context := m.Context{User: &user, Label: &label, Page: &page, Custom: &custom}

	tests := []struct {
		Transformable transform.Transformable
		Output        common.MapStr
		Msg           string
	}{
		{
			Transformable: &Event{Timestamp: timestamp},
			Output: common.MapStr{
				"agent":   common.MapStr{"name": "go", "version": "1.0"},
				"service": common.MapStr{"name": "myservice"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"user":      common.MapStr{"id": uid},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Transformable: &Event{Timestamp: timestamp, TransactionSampled: &sampledFalse},
			Output: common.MapStr{
				"transaction": common.MapStr{"sampled": false},
				"agent":       common.MapStr{"name": "go", "version": "1.0"},
				"service":     common.MapStr{"name": "myservice"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"user":      common.MapStr{"id": uid},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Transformable: &Event{Timestamp: timestamp, TransactionType: &transactionType},
			Output: common.MapStr{
				"transaction": common.MapStr{"type": "request"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"service":   common.MapStr{"name": "myservice"},
				"user":      common.MapStr{"id": uid},
				"timestamp": common.MapStr{"us": timestampUs},
				"agent":     common.MapStr{"name": "go", "version": "1.0"},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Transformable: &Event{
				Timestamp: timestamp,
				Context:   &context,
				Log:       baseLog(),
				Exception: &Exception{
					Message:    &exMsg,
					Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: "myFile"}},
				},
				TransactionId:      &trId,
				TransactionSampled: &sampledTrue,
				User:               &metadata.User{Email: &email, IP: &userIp, UserAgent: &userAgent},
				Label:              &label,
				Page:               &m.Page{Url: &url, Referer: &referer},
				Custom:             &custom,
			},

			Output: common.MapStr{
				"labels":     common.MapStr{"key": true},
				"service":    common.MapStr{"name": "myservice"},
				"agent":      common.MapStr{"name": "go", "version": "1.0"},
				"user":       common.MapStr{"email": email},
				"client":     common.MapStr{"ip": userIp},
				"user_agent": common.MapStr{"original": userAgent},
				"error": common.MapStr{
					"custom": common.MapStr{
						"foo": "bar",
					},
					"grouping_key": "1d1e44ffdf01cad5117a72fd42e4fdf4",
					"log":          common.MapStr{"message": "error log message"},
					"exception": []common.MapStr{{
						"message": "exception message",
						"stacktrace": []common.MapStr{{
							"exclude_from_grouping": false,
							"filename":              "myFile",
							"line":                  common.MapStr{"number": 0},
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
			Msg: "Payload with Event with Context.",
		},
	}

	me := metadata.NewMetadata(&service, nil, nil, &metadata.User{Id: &uid})
	tctx := &transform.Context{
		Metadata:    *me,
		Config:      transform.Config{SmapMapper: &sourcemap.SmapMapper{}},
		RequestTime: timestamp,
	}

	for idx, test := range tests {
		outputEvents := test.Transformable.Transform(tctx)
		require.Len(t, outputEvents, 1)
		outputEvent := outputEvents[0]
		assert.Equal(t, test.Output, outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
		assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
	}
}

func TestCulprit(t *testing.T) {
	c := "foo"
	fct := "fct"
	truthy := true
	st := m.Stacktrace{
		&m.StacktraceFrame{Filename: "a", Function: &fct, Sourcemap: m.Sourcemap{}},
	}
	stUpdate := m.Stacktrace{
		&m.StacktraceFrame{Filename: "a", Function: &fct, Sourcemap: m.Sourcemap{}},
		&m.StacktraceFrame{Filename: "a", LibraryFrame: &truthy, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: "f", Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: "bar", Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
	}
	mapper := sourcemap.SmapMapper{}
	tests := []struct {
		event   Event
		config  transform.Config
		culprit string
		msg     string
	}{
		{
			event:   Event{Culprit: &c},
			config:  transform.Config{},
			culprit: "foo",
			msg:     "No Sourcemap in config",
		},
		{
			event:   Event{Culprit: &c},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "foo",
			msg:     "No Stacktrace Frame given.",
		},
		{
			event:   Event{Culprit: &c, Log: &Log{Stacktrace: st}},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "foo",
			msg:     "Log.StacktraceFrame has no updated frame",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  "f",
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
			},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "f",
			msg:     "Adapt culprit to first valid Log.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Adapt culprit to first valid Exception.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Log:       &Log{Stacktrace: st},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Log and Exception StacktraceFrame given, only one changes culprit.",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  "a",
							Function:  &fct,
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{SmapMapper: &mapper},
			culprit: "a in fct",
			msg:     "Log Stacktrace is prioritized over Exception StacktraceFrame",
		},
	}
	for idx, test := range tests {
		tctx := &transform.Context{
			Config: test.config,
		}

		test.event.updateCulprit(tctx)
		assert.Equal(t, test.culprit, *test.event.Culprit,
			fmt.Sprintf("(%v) expected <%v>, received <%v>", idx, test.culprit, *test.event.Culprit))
	}
}

func TestEmptyGroupingKey(t *testing.T) {
	emptyGroupingKey := hex.EncodeToString(md5.New().Sum(nil))
	e := Event{}
	assert.Equal(t, emptyGroupingKey, e.calcGroupingKey())
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
		assert.Equal(t, groupingKey, e.calcGroupingKey(), "grouping_key mismatch", idx)
	}
}

func TestFramesUsableForGroupingKey(t *testing.T) {
	st1 := m.Stacktrace{
		&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 123, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "webpack", Lineno: 77, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "~/tmp", Lineno: 45, ExcludeFromGrouping: true},
	}
	st2 := m.Stacktrace{
		&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 123, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "webpack", Lineno: 77, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "~/tmp", Lineno: 45, ExcludeFromGrouping: false},
	}
	exMsg := "base exception"
	e1 := Event{Exception: &Exception{Message: &exMsg, Stacktrace: st1}}
	e2 := Event{Exception: &Exception{Message: &exMsg, Stacktrace: st2}}
	key1 := e1.calcGroupingKey()
	key2 := e2.calcGroupingKey()
	assert.NotEqual(t, key1, key2)
}

func TestFallbackGroupingKey(t *testing.T) {
	lineno := 12
	filename := "file"

	groupingKey := hex.EncodeToString(md5With(filename, string(lineno)))

	e := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: lineno, Filename: filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey())

	e = Event{Exception: baseException(), Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: lineno, Filename: filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey())
}

func TestNoFallbackGroupingKey(t *testing.T) {
	lineno := 1
	function := "function"
	filename := "file"
	module := "module"

	groupingKey := hex.EncodeToString(md5With(module, function))

	e := Event{
		Exception: baseException().withFrames([]*m.StacktraceFrame{
			{Lineno: lineno, Module: &module, Filename: filename, Function: &function},
		}),
	}
	assert.Equal(t, groupingKey, e.calcGroupingKey())
}

func TestGroupableEvents(t *testing.T) {
	value := "value"
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
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 57}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 57}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 0}}),
			},
			e2:     Event{},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: value}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "name"}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "name"}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "nameEx"}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 57}}),
			},
			result: true,
		},
	}

	for idx, test := range tests {
		sameGroup := test.e1.calcGroupingKey() == test.e2.calcGroupingKey()
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
	c1 := 18
	exMsg := "exception message"
	event := Event{Exception: &Exception{
		Message: &exMsg,
		Stacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 1, Colno: &c1},
		},
	}}
	tctx := &transform.Context{
		Config: transform.Config{SmapMapper: nil},
		Metadata: metadata.Metadata{
			Service: &metadata.Service{},
		},
	}
	trNoSmap := event.fields(tctx)

	event2 := Event{Exception: &Exception{
		Message: &exMsg,
		Stacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 1, Colno: &c1},
		},
	}}
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}

	tctx.Config = transform.Config{SmapMapper: &mapper}
	trWithSmap := event2.fields(tctx)

	assert.Equal(t, 1, event.Exception.Stacktrace[0].Lineno)
	assert.Equal(t, 5, event2.Exception.Stacktrace[0].Lineno)

	assert.NotEqual(t, trNoSmap["grouping_key"], trWithSmap["grouping_key"])
}

type fakeAcc struct{}

func (ac *fakeAcc) Fetch(smapId sourcemap.Id) (*s.Consumer, error) {
	file := "bundle.js.map"
	current, _ := os.Getwd()
	path := filepath.Join(current, "../../testdata/sourcemap/", file)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.Parse("", fileBytes)
}
func (a *fakeAcc) Remove(smapId sourcemap.Id) {}
