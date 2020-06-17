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

package modeldecoder

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/tests"
)

func TestErrorEventDecode(t *testing.T) {
	timestamp := json.Number("1496170407154000")
	timestampParsed := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	requestTime := time.Now()

	id, culprit, lineno := "123", "foo()", 2
	parentID, traceID, transactionID := "0123456789abcdef", "01234567890123456789abcdefabcdef", "abcdefabcdef0000"
	name, userID, email, userIP := "jane", "abc123", "j@d.com", "127.0.0.1"
	pURL, referer, origURL := "https://mypage.com", "http:mypage.com", "127.0.0.1"
	code, module, exType, handled := "200", "a", "errorEx", false
	exAttrs := map[string]interface{}{"a": "b", "c": 123, "d": map[string]interface{}{"e": "f"}}
	exMsg, logMsg, paramMsg, level, logger := "Exception Msg", "Log Msg", "log pm", "error", "mylogger"
	transactionSampled := true
	transactionType := "request"
	labels := m.Labels{"ab": "c"}
	ua := "go-1.1"
	page := m.Page{URL: m.ParseURL(pURL, ""), Referer: &referer}
	custom := m.Custom{"a": "b"}
	request := m.Req{Method: "post", Socket: &m.Socket{}, Headers: http.Header{"User-Agent": []string{ua}}, Cookies: map[string]interface{}{"a": "b"}}
	response := m.Resp{Finished: new(bool), MinimalResp: m.MinimalResp{Headers: http.Header{"Content-Type": []string{"text/html"}}}}
	h := m.Http{Request: &request, Response: &response}
	ctxURL := m.URL{Original: &origURL}
	inputMetadata := m.Metadata{
		Service: m.Service{Name: "foo"},
	}

	mergedMetadata := inputMetadata
	mergedMetadata.User = m.User{Name: name, Email: email, ID: userID}
	mergedMetadata.UserAgent.Original = ua
	mergedMetadata.Client.IP = net.ParseIP(userIP)

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"id":        id,
		"exception": map[string]interface{}{"message": exMsg},
	}

	for name, test := range map[string]struct {
		input map[string]interface{}
		cfg   Config
		e     *m.Error
	}{
		"minimal valid error": {
			input: map[string]interface{}{},
			e: &m.Error{
				Metadata:  inputMetadata,
				ID:        &id,
				Timestamp: requestTime,
				Exception: &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
			},
		},
		"minimal valid error with specified timestamp": {
			input: map[string]interface{}{"timestamp": timestamp},
			e: &m.Error{
				Metadata:  inputMetadata,
				ID:        &id,
				Timestamp: timestampParsed,
				Exception: &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
			},
		},
		"minimal valid error with log and exception": {
			input: map[string]interface{}{
				"exception": map[string]interface{}{"message": exMsg},
				"log":       map[string]interface{}{"message": logMsg},
			},
			e: &m.Error{
				Metadata:  inputMetadata,
				ID:        &id,
				Timestamp: requestTime,
				Exception: &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
				Log:       &m.Log{Message: logMsg, Stacktrace: m.Stacktrace{}},
			},
		},
		"valid error experimental=true, no experimental payload": {
			input: map[string]interface{}{
				"context": map[string]interface{}{"foo": []string{"a", "b"}},
			},
			e: &m.Error{
				Metadata:  inputMetadata,
				ID:        &id,
				Timestamp: requestTime,
				Exception: &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
			},
			cfg: Config{Experimental: true},
		},
		"valid error experimental=false": {
			input: map[string]interface{}{
				"context": map[string]interface{}{"experimental": []string{"a", "b"}},
			},
			e: &m.Error{
				Metadata:  inputMetadata,
				ID:        &id,
				Timestamp: requestTime,
				Exception: &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
			},
			cfg: Config{Experimental: false},
		},
		"valid error experimental=true": {
			input: map[string]interface{}{
				"context": map[string]interface{}{"experimental": []string{"a", "b"}},
			},
			e: &m.Error{
				Metadata:     inputMetadata,
				ID:           &id,
				Timestamp:    requestTime,
				Exception:    &m.Exception{Message: &exMsg, Stacktrace: m.Stacktrace{}},
				Experimental: []string{"a", "b"},
			},
			cfg: Config{Experimental: true},
		},
		"full valid error event": {
			input: map[string]interface{}{
				"timestamp": timestamp,
				"context": map[string]interface{}{
					"a":      "b",
					"user":   map[string]interface{}{"username": name, "email": email, "ip": userIP, "id": userID},
					"tags":   map[string]interface{}{"ab": "c"},
					"page":   map[string]interface{}{"url": pURL, "referer": referer},
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
					"message":    exMsg,
					"code":       code,
					"module":     module,
					"attributes": exAttrs,
					"type":       exType,
					"handled":    handled,
					"stacktrace": []interface{}{
						map[string]interface{}{
							"filename": "file",
						},
					},
				},
				"log": map[string]interface{}{
					"message":       logMsg,
					"param_message": paramMsg,
					"level":         level, "logger_name": logger,
					"stacktrace": []interface{}{
						map[string]interface{}{
							"filename": "log file", "lineno": 2.0,
						},
					},
				},
				"id":             id,
				"transaction_id": transactionID,
				"parent_id":      parentID,
				"trace_id":       traceID,
				"culprit":        culprit,
				"transaction":    map[string]interface{}{"sampled": transactionSampled, "type": transactionType},
			},
			e: &m.Error{
				Metadata:  mergedMetadata,
				Timestamp: timestampParsed,
				Labels:    &labels,
				Page:      &page,
				Custom:    &custom,
				HTTP:      &h,
				URL:       &ctxURL,
				Exception: &m.Exception{
					Message:    &exMsg,
					Code:       code,
					Type:       &exType,
					Module:     &module,
					Attributes: exAttrs,
					Handled:    &handled,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: tests.StringPtr("file")},
					},
				},
				Log: &m.Log{
					Message:      logMsg,
					ParamMessage: &paramMsg,
					Level:        &level,
					LoggerName:   &logger,
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{Filename: tests.StringPtr("log file"), Lineno: &lineno},
					},
				},
				ID:                 &id,
				TransactionID:      transactionID,
				TransactionSampled: &transactionSampled,
				TransactionType:    &transactionType,
				ParentID:           parentID,
				TraceID:            traceID,
				Culprit:            &culprit,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				if v == nil {
					delete(input, k)
				} else {
					input[k] = v
				}
			}
			batch := &m.Batch{}
			err := DecodeError(Input{
				Raw:         input,
				RequestTime: requestTime,
				Metadata:    inputMetadata,
				Config:      test.cfg,
			}, batch)
			require.NoError(t, err)
			assert.Equal(t, test.e, batch.Errors[0])
		})
	}
}

func TestErrorEventDecodeInvalid(t *testing.T) {
	err := DecodeError(Input{Raw: nil}, &m.Batch{})
	require.EqualError(t, err, "failed to validate error: error validating JSON: input missing")

	err = DecodeError(Input{Raw: ""}, &m.Batch{})
	require.EqualError(t, err, "failed to validate error: error validating JSON: invalid input type")

	// baseInput holds the minimal valid input. Test-specific input is added to this.
	baseInput := map[string]interface{}{
		"id": "id",
		"exception": map[string]interface{}{
			"message": "message",
		},
	}
	err = DecodeError(Input{Raw: baseInput}, &m.Batch{})
	require.NoError(t, err)

	for name, test := range map[string]struct {
		input map[string]interface{}
		e     *m.Error
	}{
		"error decoding timestamp": {
			input: map[string]interface{}{"timestamp": 123},
		},
		"error decoding transaction id": {
			input: map[string]interface{}{"transaction_id": 123},
		},
		"parent id given, but no trace id": {
			input: map[string]interface{}{"parent_id": "123"},
		},
		"trace id given, but no parent id": {
			input: map[string]interface{}{"trace_id": "123"},
		},
		"invalid type for exception stacktrace": {
			input: map[string]interface{}{
				"exception": map[string]interface{}{
					"message":    "Exception Msg",
					"stacktrace": "123",
				},
			},
		},
		"invalid type for log stacktrace": {
			input: map[string]interface{}{
				"exception": nil,
				"log": map[string]interface{}{
					"message":    "Log Msg",
					"stacktrace": "123",
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := make(map[string]interface{})
			for k, v := range baseInput {
				input[k] = v
			}
			for k, v := range test.input {
				if v == nil {
					delete(input, k)
				} else {
					input[k] = v
				}
			}
			err := DecodeError(Input{Raw: input}, &m.Batch{})
			require.Error(t, err)
			t.Logf("%s", err)
		})
	}
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
		result := &m.Batch{}
		err := DecodeError(Input{Raw: badID}, result)
		assert.Error(t, err)
		assert.Nil(t, result.Errors)
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
		result := &m.Batch{}
		err := DecodeError(Input{Raw: badException}, result)
		assert.Error(t, err)
		assert.Nil(t, result.Errors)
	})

	t.Run("wrong cause type", func(t *testing.T) {
		badException := map[string]interface{}{
			"id": "id",
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
				"cause":   []interface{}{7.4},
			},
		}
		err := DecodeError(Input{Raw: badException}, &m.Batch{})
		require.Error(t, err)
		assert.Regexp(t, "failed to validate error:(.|\n)*properties/cause/items/type(.|\n)*expected object or null, but got number", err.Error())
	})

	t.Run("handle nil exceptions", func(t *testing.T) {
		emptyCauses := map[string]interface{}{
			"exception": map[string]interface{}{
				"message": "message0",
				"type":    "type0",
				"cause": []interface{}{
					map[string]interface{}{"message": "message1", "type": "type1", "cause": []interface{}{}},
					map[string]interface{}{},
				},
			},
		}
		err := DecodeError(Input{Raw: emptyCauses}, &m.Batch{})
		require.Error(t, err)
		assert.Regexp(t, "failed to validate error:(.|\n)* missing properties: \"id\"", err.Error())
	})
}
