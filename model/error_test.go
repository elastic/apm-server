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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/sourcemap/test"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/transform"

	"github.com/elastic/beats/v7/libbeat/common"
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

func (e *Exception) withFrames(frames []*StacktraceFrame) *Exception {
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

func (l *Log) withFrames(frames []*StacktraceFrame) *Log {
	l.Stacktrace = frames
	return l
}

func TestHandleExceptionTree(t *testing.T) {
	event := &Error{
		ID: tests.StringPtr("id"),
		Exception: &Exception{
			Message: tests.StringPtr("message0"),
			Type:    tests.StringPtr("type0"),
			Stacktrace: Stacktrace{{
				Filename: tests.StringPtr("file0"),
			}},
			Cause: []Exception{{
				Message: tests.StringPtr("message1"),
				Type:    tests.StringPtr("type1"),
			}, {
				Message: tests.StringPtr("message2"),
				Type:    tests.StringPtr("type2"),
				Cause: []Exception{{
					Message: tests.StringPtr("message3"),
					Type:    tests.StringPtr("type3"),
					Cause: []Exception{{
						Message: tests.StringPtr("message4"),
						Type:    tests.StringPtr("type4"),
					}, {
						Message: tests.StringPtr("message5"),
						Type:    tests.StringPtr("type5"),
					}},
				}},
			}, {
				Message: tests.StringPtr("message6"),
				Type:    tests.StringPtr("type6"),
			}},
		},
	}
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
		Stacktrace: []*StacktraceFrame{{Filename: tests.StringPtr("st file")}},
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
	trID := "945254c5-67a5-417e-8a4e-aa29efcbfb79"

	tests := map[string]struct {
		Error  Error
		Output common.MapStr
	}{
		"minimal": {
			Error: Error{},
			Output: common.MapStr{
				"grouping_key": hex.EncodeToString(md5.New().Sum(nil)),
			},
		},
		"withLog": {
			Error: Error{Log: baseLog()},
			Output: common.MapStr{
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseLogGroupingKey,
			},
		},
		"withLogAndException": {
			Error: Error{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"withException": {
			Error: Error{Exception: baseException()},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"stringCode": {
			Error: Error{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"intCode": {
			Error: Error{Exception: baseException().withCode(13)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"floatCode": {
			Error: Error{Exception: baseException().withCode(13.0)},
			Output: common.MapStr{
				"exception":    []common.MapStr{{"message": "exception message", "code": "13"}},
				"grouping_key": baseExceptionGroupingKey,
			},
		},
		"withFrames": {
			Error: Error{
				ID:            &id,
				Timestamp:     time.Now(),
				Culprit:       &culprit,
				Exception:     &exception,
				Log:           &log,
				TransactionID: trID,
				RUM:           true,

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
			output := tc.Error.Transform(context.Background(), &transform.Config{
				RUM: transform.RUMConfig{SourcemapStore: &sourcemap.Store{}},
			})
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
	exMsg := "exception message"
	trID := "945254c5-67a5-417e-8a4e-aa29efcbfb79"
	sampledTrue, sampledFalse := true, false
	transactionType := "request"

	email, userIP, userAgent := "m@m.com", "127.0.0.1", "js-1.0"
	uid := "1234567889"
	url, referer := "https://localhost", "http://localhost"
	labels := common.MapStr{"key": true}
	custom := Custom(common.MapStr{"foo": "bar"})

	serviceName, agentName, version := "myservice", "go", "1.0"
	md := Metadata{
		Service: Service{
			Name: serviceName, Version: version,
			Agent: Agent{Name: agentName, Version: version},
		},
		Labels: common.MapStr{"label": 101},
	}

	mdWithContext := md
	mdWithContext.User = User{ID: uid, Email: email}
	mdWithContext.Client.IP = net.ParseIP(userIP)
	mdWithContext.UserAgent.Original = userAgent

	for name, tc := range map[string]struct {
		Transformable transform.Transformable
		Output        common.MapStr
		Msg           string
	}{
		"valid": {
			Transformable: &Error{Timestamp: timestamp, Metadata: md},
			Output: common.MapStr{
				"data_stream.type":    "logs",
				"data_stream.dataset": "apm.error.myservice",
				"agent":               common.MapStr{"name": "go", "version": "1.0"},
				"service":             common.MapStr{"name": "myservice", "version": "1.0"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"notSampled": {
			Transformable: &Error{Timestamp: timestamp, Metadata: md, TransactionSampled: &sampledFalse},
			Output: common.MapStr{
				"data_stream.type":    "logs",
				"data_stream.dataset": "apm.error.myservice",
				"transaction":         common.MapStr{"sampled": false},
				"agent":               common.MapStr{"name": "go", "version": "1.0"},
				"service":             common.MapStr{"name": "myservice", "version": "1.0"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"timestamp": common.MapStr{"us": timestampUs},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"withMeta": {
			Transformable: &Error{Timestamp: timestamp, Metadata: md, TransactionType: &transactionType},
			Output: common.MapStr{
				"data_stream.type":    "logs",
				"data_stream.dataset": "apm.error.myservice",
				"transaction":         common.MapStr{"type": "request"},
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"service":   common.MapStr{"name": "myservice", "version": "1.0"},
				"timestamp": common.MapStr{"us": timestampUs},
				"agent":     common.MapStr{"name": "go", "version": "1.0"},
				"labels":    common.MapStr{"label": 101},
			},
		},
		"withContext": {
			Transformable: &Error{
				Timestamp: timestamp,
				Metadata:  mdWithContext,
				Log:       baseLog(),
				Exception: &Exception{
					Message:    &exMsg,
					Stacktrace: Stacktrace{&StacktraceFrame{Filename: tests.StringPtr("myFile")}},
				},
				TransactionID:      trID,
				TransactionSampled: &sampledTrue,
				Labels:             labels,
				Page:               &Page{URL: &URL{Original: &url}, Referer: &referer},
				Custom:             &custom,
				RUM:                true,
			},

			Output: common.MapStr{
				"data_stream.type":    "logs",
				"data_stream.dataset": "apm.error.myservice",
				"labels":              common.MapStr{"key": true, "label": 101},
				"service":             common.MapStr{"name": "myservice", "version": "1.0"},
				"agent":               common.MapStr{"name": "go", "version": "1.0"},
				"user":                common.MapStr{"id": uid, "email": email},
				"client":              common.MapStr{"ip": userIP},
				"source":              common.MapStr{"ip": userIP},
				"user_agent":          common.MapStr{"original": userAgent},
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
			outputEvents := tc.Transformable.Transform(context.Background(), &transform.Config{
				DataStreams: true,
				RUM:         transform.RUMConfig{SourcemapStore: &sourcemap.Store{}},
			})
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
	st := Stacktrace{
		&StacktraceFrame{Filename: tests.StringPtr("a"), Function: &fct},
	}
	stUpdate := Stacktrace{
		&StacktraceFrame{Filename: tests.StringPtr("a"), Function: &fct},
		&StacktraceFrame{Filename: tests.StringPtr("a"), LibraryFrame: &truthy, SourcemapUpdated: &truthy},
		&StacktraceFrame{Filename: tests.StringPtr("f"), Function: &fct, SourcemapUpdated: &truthy},
		&StacktraceFrame{Filename: tests.StringPtr("bar"), Function: &fct, SourcemapUpdated: &truthy},
	}
	store := &sourcemap.Store{}
	tests := []struct {
		event   Error
		config  transform.Config
		culprit string
		msg     string
	}{
		{
			event:   Error{Culprit: &c},
			config:  transform.Config{},
			culprit: "foo",
			msg:     "No Sourcemap in config",
		},
		{
			event:   Error{Culprit: &c},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "foo",
			msg:     "No Stacktrace Frame given.",
		},
		{
			event:   Error{Culprit: &c, Log: &Log{Stacktrace: st}},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "foo",
			msg:     "Log.StacktraceFrame has no updated frame",
		},
		{
			event: Error{
				Culprit: &c,
				Log: &Log{
					Stacktrace: Stacktrace{
						&StacktraceFrame{
							Filename:         tests.StringPtr("f"),
							Classname:        tests.StringPtr("xyz"),
							SourcemapUpdated: &truthy,
						},
					},
				},
			},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "f",
			msg:     "Adapt culprit to first valid Log.StacktraceFrame filename information.",
		},
		{
			event: Error{
				Culprit: &c,
				Log: &Log{
					Stacktrace: Stacktrace{
						&StacktraceFrame{
							Classname:        tests.StringPtr("xyz"),
							SourcemapUpdated: &truthy,
						},
					},
				},
			},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "xyz",
			msg:     "Adapt culprit Log.StacktraceFrame classname information.",
		},
		{
			event: Error{
				Culprit:   &c,
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "f in fct",
			msg:     "Adapt culprit to first valid Exception.StacktraceFrame information.",
		},
		{
			event: Error{
				Culprit:   &c,
				Log:       &Log{Stacktrace: st},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "f in fct",
			msg:     "Log and Exception StacktraceFrame given, only one changes culprit.",
		},
		{
			event: Error{
				Culprit: &c,
				Log: &Log{
					Stacktrace: Stacktrace{
						&StacktraceFrame{
							Filename:         tests.StringPtr("a"),
							Function:         &fct,
							SourcemapUpdated: &truthy,
						},
					},
				},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}},
			culprit: "a in fct",
			msg:     "Log Stacktrace is prioritized over Exception StacktraceFrame",
		},
	}
	for idx, test := range tests {
		t.Run(fmt.Sprint(idx), func(t *testing.T) {

			test.event.updateCulprit(&test.config)
			assert.Equal(t, test.culprit, *test.event.Culprit,
				fmt.Sprintf("(%v) %s: expected <%v>, received <%v>", idx, test.msg, test.culprit, *test.event.Culprit))
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
				ID: &id,
				Page: &Page{
					URL:     ParseURL(urlExample, ""),
					Referer: nil,
				},
			},
			Output: common.MapStr{
				"domain":   "example.com",
				"full":     "http://example.com/path",
				"original": "http://example.com/path",
				"path":     "/path",
				"scheme":   "http",
			},
			Msg: "With page URL",
		},
		{
			Error: Error{
				ID:        &id,
				Timestamp: time.Now(),
				URL:       ParseURL("https://localhost:8200/", ""),
				Page: &Page{
					URL:     ParseURL(urlExample, ""),
					Referer: nil,
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
		output := test.Error.Transform(context.Background(), &transform.Config{})
		assert.Equal(t, test.Output, output[0].Fields["url"], fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEmptyGroupingKey(t *testing.T) {
	emptyGroupingKey := hex.EncodeToString(md5.New().Sum(nil))
	e := Error{}
	assert.Equal(t, emptyGroupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestExplicitGroupingKey(t *testing.T) {
	attr := "hello world"
	diffAttr := "huhu"

	groupingKey := hex.EncodeToString(md5With(attr))

	e1 := Error{Log: baseLog().withParamMsg(attr)}
	e2 := Error{Exception: baseException().withType(attr)}
	e3 := Error{Log: baseLog().withFrames([]*StacktraceFrame{{Function: &attr}})}
	e4 := Error{Exception: baseException().withFrames([]*StacktraceFrame{{Function: &attr}})}
	e5 := Error{
		Log:       baseLog().withFrames([]*StacktraceFrame{{Function: &diffAttr}}),
		Exception: baseException().withFrames([]*StacktraceFrame{{Function: &attr}}),
	}

	for idx, e := range []Error{e1, e2, e3, e4, e5} {
		assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)), "grouping_key mismatch", idx)
	}
}

func TestFramesUsableForGroupingKey(t *testing.T) {
	webpackLineno := 77
	tmpLineno := 45
	st1 := Stacktrace{
		&StacktraceFrame{Filename: tests.StringPtr("/a/b/c"), ExcludeFromGrouping: false},
		&StacktraceFrame{Filename: tests.StringPtr("webpack"), Lineno: &webpackLineno, ExcludeFromGrouping: false},
		&StacktraceFrame{Filename: tests.StringPtr("~/tmp"), Lineno: &tmpLineno, ExcludeFromGrouping: true},
	}
	st2 := Stacktrace{
		&StacktraceFrame{Filename: tests.StringPtr("/a/b/c"), ExcludeFromGrouping: false},
		&StacktraceFrame{Filename: tests.StringPtr("webpack"), Lineno: &webpackLineno, ExcludeFromGrouping: false},
		&StacktraceFrame{Filename: tests.StringPtr("~/tmp"), Lineno: &tmpLineno, ExcludeFromGrouping: false},
	}
	exMsg := "base exception"
	e1 := Error{Exception: &Exception{Message: &exMsg, Stacktrace: st1}}
	e2 := Error{Exception: &Exception{Message: &exMsg, Stacktrace: st2}}
	key1 := e1.calcGroupingKey(flattenExceptionTree(e1.Exception))
	key2 := e2.calcGroupingKey(flattenExceptionTree(e2.Exception))
	assert.NotEqual(t, key1, key2)
}

func TestFallbackGroupingKey(t *testing.T) {
	lineno := 12
	filename := "file"

	groupingKey := hex.EncodeToString(md5With(filename))

	e := Error{Exception: baseException().withFrames([]*StacktraceFrame{{Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))

	e = Error{Exception: baseException(), Log: baseLog().withFrames([]*StacktraceFrame{{Lineno: &lineno, Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestNoFallbackGroupingKey(t *testing.T) {
	lineno := 1
	function := "function"
	filename := "file"
	module := "module"

	groupingKey := hex.EncodeToString(md5With(module, function))

	e := Error{
		Exception: baseException().withFrames([]*StacktraceFrame{
			{Lineno: &lineno, Module: &module, Filename: &filename, Function: &function},
		}),
	}
	assert.Equal(t, groupingKey, e.calcGroupingKey(flattenExceptionTree(e.Exception)))
}

func TestGroupableEvents(t *testing.T) {
	value := "value"
	name := "name"
	var tests = []struct {
		e1     Error
		e2     Error
		result bool
	}{
		{
			e1: Error{
				Log: baseLog().withParamMsg(value),
			},
			e2: Error{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Error{
				Exception: baseException().withType(value),
			},
			e2: Error{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Error{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Error{
				Log: baseLog().withParamMsg(value),
			},
			result: false,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Function: &value}}),
			},
			e2: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Function: &value}}),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{}),
			},
			e2: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{}),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{}),
			},
			e2:     Error{},
			result: false,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Module: &value}}),
			},
			e2: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Filename: &value}}),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Filename: &name}}),
			},
			e2: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Module: &value, Filename: &name}}),
			},
			result: false,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Module: &value, Filename: &name}}),
			},
			e2: Error{
				Exception: baseException().withFrames([]*StacktraceFrame{{Module: &value, Filename: tests.StringPtr("nameEx")}}),
			},
			result: true,
		},
		{
			e1: Error{
				Log: baseLog().withFrames([]*StacktraceFrame{{Filename: &name}}),
			},
			e2: Error{
				Exception: baseException().withFrames([]*StacktraceFrame{{Filename: &name}}),
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
	event := Error{
		Metadata: Metadata{
			Service: Service{
				Name:    "foo",
				Version: "bar",
			},
		},
		Exception: &Exception{
			Message: tests.StringPtr("exception message"),
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Filename: tests.StringPtr("/a/b/c"),
					Lineno:   tests.IntPtr(1),
					Colno:    tests.IntPtr(23),
					AbsPath:  tests.StringPtr("../a/b"),
				},
			},
		},
		RUM: true,
	}

	// transform without sourcemap store
	transformedNoSourcemap := event.fields(context.Background(), &transform.Config{})
	assert.Equal(t, 1, *event.Exception.Stacktrace[0].Lineno)

	// transform with sourcemap store
	store, err := sourcemap.NewStore(test.ESClientWithValidSourcemap(t), "apm-*sourcemap*", time.Minute)
	require.NoError(t, err)
	transformedWithSourcemap := event.fields(context.Background(), &transform.Config{
		RUM: transform.RUMConfig{SourcemapStore: store},
	})
	assert.Equal(t, 5, *event.Exception.Stacktrace[0].Lineno)

	assert.NotEqual(t, transformedNoSourcemap["exception"], transformedWithSourcemap["exception"])
	assert.NotEqual(t, transformedNoSourcemap["grouping_key"], transformedWithSourcemap["grouping_key"])
}
