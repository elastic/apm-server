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

package v2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/v7/libbeat/common"
)

//
// Test Validation rules
//

func TestUserValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "id-string", data: `{"id":"user123"}`},
		{name: "id-int", data: `{"id":44}`},
		{name: "id-float", errorKey: "inputTypes", data: `{"id":45.6}`},
		{name: "id-bool", errorKey: "inputTypes", data: `{"id":true}`},
		{name: "id-string-max-len", data: `{"id":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "id-string-max-len-exceeded", errorKey: "max", data: `{"id":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "metadata", testcases, "user")
	testValidation(t, "transaction", testcases, "context", "user")
}

func TestServiceValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "service-name-az", data: `{"agent":{"name":"go","version":"1.0"},"name":"abcdefghijklmnopqrstuvwxyz"}`},
		{name: "service-name-AZ", data: `{"agent":{"name":"go","version":"1.0"},"name":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"}`},
		{name: "service-name-09 _-", data: `{"agent":{"name":"go","version":"1.0"},"name":"0123456789 -_"}`},
		{name: "service-name-invalid", errorKey: "patternAlphaNumericExt",
			data: `{"agent":{"name":"go","version":"1.0"},"name":"⌘"}`},
		{name: "service-name-max", data: `{"agent":{"name":"go","version":"1.0"},"name":"` + modeldecodertest.BuildStringWith(1024, '-') + `"}`},
		{name: "service-name-max-exceeded", errorKey: "max",
			data: `{"agent":{"name":"go","version":"1.0"},"name":"` + modeldecodertest.BuildStringWith(1025, '-') + `"}`},
	}
	testValidation(t, "metadata", testcases, "service")
	testValidation(t, "transaction", testcases, "context", "service")
	testValidation(t, "span", testcases, "context", "service")
}

func TestLabelValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "valid", data: `{"k1\".*":"v1.s*\"","k2":2.3,"k3":3,"k4":true,"k5":null}`},
		{name: "restricted-type", errorKey: "inputTypesVals", data: `{"k1":{"k2":"v1"}}`},
		{name: "restricted-type-slice", errorKey: "inputTypesVals", data: `{"k1":{"v1":[1,2,3]}}`},
		{name: "max-len", data: `{"k1":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "max-len-exceeded", errorKey: "maxLengthVals", data: `{"k1":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "metadata", testcases, "labels")
	testValidation(t, "transaction", testcases, "context", "tags")
	testValidation(t, "span", testcases, "context", "tags")
	testValidation(t, "metricset", testcases, "tags")
}

func TestSamplesValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "valid", data: `{"k.1\\":{"value": 34.5},"k.2.a":{"value":5}}`},
		{name: "key-asterisk", errorKey: "patternNoAsteriskQuote", data: `{"k1*":{"value": 34.5},"k.2.a":{"value":5}}`},
		{name: "key-quotemark", errorKey: "patternNoAsteriskQuote", data: `{"k1\"":{"value": 34.5}}`},
	}
	testValidation(t, "metricset", testcases, "samples")
}

func TestMaxLenValidationRules(t *testing.T) {
	// this tests an arbitrary field to ensure the `max` rule on strings works as expected
	testcases := []testcase{
		{name: "title-max-len", data: `{"pid":1,"title":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "title-max-len-exceeded", errorKey: "max",
			data: `{"pid":1,"title":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "metadata", testcases, "process")
}

func TestContextValidationRules(t *testing.T) {
	t.Run("custom", func(t *testing.T) {
		testcases := []testcase{
			{name: "custom", data: `{"custom":{"k\\1":{"v.1":123,"v*2":"value\\"},"k\"2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
		}
		testValidation(t, "transaction", testcases, "context")
	})

	t.Run("request", func(t *testing.T) {
		testcases := []testcase{
			{name: "request-body-string", data: `{"request":{"method":"get","body":"value"}}`},
			{name: "request-body-object", data: `{"request":{"method":"get","body":{"a":"b"}}}`},
			{name: "request-body-array", errorKey: "body",
				data: `{"request":{"method":"get","body":[1,2]}}`},
		}
		testValidation(t, "transaction", testcases, "context")
	})
}

func TestDurationValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "duration", data: `0.0`},
		{name: "duration", errorKey: "min", data: `-0.09`},
	}
	testValidation(t, "transaction", testcases, "duration")
}

func TestMarksValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "marks", data: `{"k.1*\\\"":{"v.1*\\\"":12.3}}`},
	}
	testValidation(t, "transaction", testcases, "marks")
}

func TestOutcomeValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "outcome-success", data: `"success"`},
		{name: "outcome-failure", data: `"failure"`},
		{name: "outcome-unknown", data: `"unknown"`},
		{name: "outcome-invalid", errorKey: "enum", data: `"anything"`},
	}
	testValidation(t, "transaction", testcases, "outcome")
	testValidation(t, "span", testcases, "outcome")
}

func TestURLValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "port-string", data: `{"request":{"method":"get","url":{"port":"8200"}}}`},
		{name: "port-int", data: `{"request":{"method":"get","url":{"port":8200}}}`},
		{name: "port-invalid-string", errorKey: "targetType",
			data: `{"request":{"method":"get","url":{"port":"invalid"}}}`},
		{name: "port-invalid-type", errorKey: "inputTypes",
			data: `{"request":{"method":"get","url":{"port":[8200,8201]}}}`},
		{name: "port-invalid-type", errorKey: "inputTypes",
			data: `{"request":{"method":"get","url":{"port":{"val":8200}}}}`},
	}
	testValidation(t, "transaction", testcases, "context")
}

//
// Test Reset()
//

func TestReset(t *testing.T) {
	addStr := func(s string) nullable.String {
		ns := nullable.String{}
		ns.Set(s)
		return ns
	}
	decode := func(t *testing.T, inp string, out interface{}) {
		require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(&out))
	}
	t.Run("struct", func(t *testing.T) {
		var out metadataServiceNode
		inputs := []string{`{"configured_name":"a"}`, `{"configured_name":"b"}`, `{}`}
		expected := []metadataServiceNode{{Name: addStr("a")}, {Name: addStr("b")}, {}}
		for i := 0; i < len(inputs); i++ {
			out.Reset()
			decode(t, inputs[i], &out)
			assert.Equal(t, expected[i], out)
		}
	})
	t.Run("map", func(t *testing.T) {
		var out metadata
		inputs := []string{
			`{"labels":{"a":"1","b":"s","c":true}}}`,
			`{"labels":{"a":"new"}}}`,
			`{}`}
		expected := []metadata{
			{Labels: common.MapStr{"a": "1", "b": "s", "c": true}},
			{Labels: common.MapStr{"a": "new"}},
			{Labels: common.MapStr{}}}
		for i := 0; i < len(inputs); i++ {
			out.Reset()
			decode(t, inputs[i], &out)
			assert.Equal(t, expected[i], out)
		}
	})
	t.Run("map-structs", func(t *testing.T) {
		var out transaction
		inputs := []string{
			`{"marks":{"group1":{"meas1":43.5,"meas2":23.5},"group2":{"a":1,"b":14}}}`,
			`{"marks":{"group1":{"meas1":14}}}`,
			`{}`,
		}
		expected := []transaction{
			{Marks: transactionMarks{Events: map[string]transactionMarkEvents{
				"group1": {Measurements: map[string]float64{"meas1": 43.5, "meas2": 23.5}},
				"group2": {Measurements: map[string]float64{"a": 1, "b": 14}}}}},
			{Marks: transactionMarks{Events: map[string]transactionMarkEvents{
				"group1": {Measurements: map[string]float64{"meas1": 14}}}}},
			{Marks: transactionMarks{Events: map[string]transactionMarkEvents{}}}}
		for i := 0; i < len(inputs); i++ {
			out.Reset()
			decode(t, inputs[i], &out)
			assert.Equal(t, expected[i], out)
		}
	})
	t.Run("slice", func(t *testing.T) {
		var out metadataProcess
		inputs := []string{
			`{"argv":["a","b"]}`,
			`{"argv":["c"]}`,
			`{}`}
		expected := []metadataProcess{
			{Argv: []string{"a", "b"}},
			{Argv: []string{"c"}},
			{Argv: []string{}}}
		for i := 0; i < len(inputs); i++ {
			out.Reset()
			decode(t, inputs[i], &out)
			assert.Equal(t, expected[i], out)
		}
	})
	t.Run("slice-structs", func(t *testing.T) {
		var out errorEvent
		inputs := []string{
			`{"exception":{"message":"bang","cause":[{"message":"a","type":"runtime ex","cause":[{"message":"inner"}]},{"message":"b"}]},"log":{"message":"boom","stacktrace":[{"classname":"User::Common","filename":"a","post_context":["line4","line5"]},{"classname":"ABC","filename":"b"}]}}`,
			`{"exception":{"message":"boom","cause":[{"message":"c","cause":[{"type":"a"}]}]},"log":{"message":"boom","stacktrace":[{"filename":"b"}]}}`,
			`{}`}
		expected := []errorEvent{
			{Exception: errorException{
				Message: addStr("bang"),
				Cause: []errorException{
					{Message: addStr("a"), Type: addStr("runtime ex"), Cause: []errorException{{Message: addStr("inner")}}},
					{Message: addStr("b")}}},
				Log: errorLog{Message: addStr("boom"), Stacktrace: []stacktraceFrame{
					{Classname: addStr("User::Common"), Filename: addStr("a"), PostContext: []string{"line4", "line5"}},
					{Classname: addStr("ABC"), Filename: addStr("b")}}}},
			{Exception: errorException{
				Message: addStr("boom"),
				Cause: []errorException{
					{Message: addStr("c"), Cause: []errorException{{Type: addStr("a")}}}}},
				Log: errorLog{Message: addStr("boom"), Stacktrace: []stacktraceFrame{
					{Filename: addStr("b"), PostContext: []string{}}}}},
			{Exception: errorException{Cause: []errorException{}}, Log: errorLog{Stacktrace: []stacktraceFrame{}}}}
		for i := 0; i < len(inputs); i++ {
			out.Reset()
			decode(t, inputs[i], &out)
			assert.Equal(t, expected[i], out)
		}
	})
}

//
// Test Required fields
//

func TestErrorRequiredValidationRules(t *testing.T) {
	// setup: create full struct with arbitrary values set
	var event errorEvent
	modeldecodertest.InitStructValues(&event)
	// test vanilla struct is valid
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"context.request.method":               nil,
		"context.destination.service.resource": nil,
		"context.destination.service.type":     nil,
		"id":                                   nil,
		"log.message":                          nil,
		"parent_id":                            nil, //requiredIf
		"trace_id":                             nil, //requiredIf
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

func TestErrorRequiredOneOfValidationRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setupFn func(event *errorEvent)
	}{
		{name: "all", setupFn: func(e *errorEvent) {
			e.Log = errorLog{}
			e.Log.Message.Set("test message")
			e.Exception = errorException{}
			e.Exception.Message.Set("test message")
		}},
		{name: "log", setupFn: func(e *errorEvent) {
			e.Log = errorLog{}
			e.Log.Message.Set("test message")
		}},
		{name: "exception/message", setupFn: func(e *errorEvent) {
			e.Exception = errorException{}
			e.Exception.Message.Set("test message")
		}},
		{name: "exception/type", setupFn: func(e *errorEvent) {
			e.Exception = errorException{}
			e.Exception.Type.Set("test type")
		}},
		{name: "exception/cause",
			setupFn: func(e *errorEvent) {
				exception := errorException{}
				exception.Type.Set("test type")
				cause := errorException{}
				cause.Type.Set("cause type")
				exception.Cause = []errorException{cause}
				e.Exception = exception
			},
		},
		{name: "*/stacktrace/classname", setupFn: func(e *errorEvent) {
			e.Log = errorLog{}
			e.Log.Message.Set("test message")
			frame := stacktraceFrame{}
			frame.Classname.Set("myClass")
			e.Log.Stacktrace = []stacktraceFrame{frame}
		}},
		{name: "*/stacktrace/filename", setupFn: func(e *errorEvent) {
			e.Exception = errorException{}
			e.Exception.Message.Set("test message")
			frame := stacktraceFrame{}
			frame.Filename.Set("myFile")
			e.Exception.Stacktrace = []stacktraceFrame{frame}
		}},
	} {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			var event errorEvent
			event.ID.Set("123")
			tc.setupFn(&event)
			require.NoError(t, event.validate())
		})
	}

	for _, tc := range []struct {
		name    string
		err     string
		setupFn func(event *errorEvent)
	}{
		{name: "error",
			err:     "requires at least one of the fields 'exception;log'",
			setupFn: func(e *errorEvent) {}},
		{name: "exception",
			err: "exception: requires at least one of the fields 'message;type'",
			setupFn: func(e *errorEvent) {
				exception := errorException{}
				exception.Handled.Set(true)
				e.Exception = exception
			},
		},
		{name: "exception/cause",
			err: "exception: cause: requires at least one of the fields 'message;type'",
			setupFn: func(e *errorEvent) {
				exception := errorException{}
				exception.Type.Set("test type")
				cause := errorException{}
				cause.Code.Set("400")
				exception.Cause = []errorException{cause}
				e.Exception = exception
			},
		},
		{name: "*/stacktrace",
			err: "log: stacktrace: requires at least one of the fields 'classname;filename'",
			setupFn: func(e *errorEvent) {
				frame := stacktraceFrame{}
				frame.LibraryFrame.Set(false)
				log := errorLog{}
				log.Message.Set("true")
				log.Stacktrace = []stacktraceFrame{frame}
				e.Log = log
			},
		},
	} {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			var event errorEvent
			event.ID.Set("123")
			tc.setupFn(&event)
			err := event.validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
		})
	}
}

func TestErrorRequiredIfAnyValidationRules(t *testing.T) {
	validErrorEvent := func() errorEvent {
		var event errorEvent
		event.ID.Set("123")
		event.Exception = errorException{}
		event.Exception.Message.Set("test message")
		return event
	}
	for _, tc := range []struct {
		name    string
		setupFn func(event *errorEvent)
	}{
		{name: "traceID-nil", setupFn: func(*errorEvent) {}},
		{name: "traceID-parentID-transactionID", setupFn: func(e *errorEvent) {
			e.TraceID.Set("abcd")
			e.ParentID.Set("xxx")
			e.TransactionID.Set("xxx")
		}},
		{name: "traceID-parentID", setupFn: func(e *errorEvent) {
			e.TraceID.Set("abcd")
			e.ParentID.Set("xxx")
		}},
	} {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			event := validErrorEvent()
			tc.setupFn(&event)
			require.NoError(t, event.validate())
		})
	}

	for _, tc := range []struct {
		name    string
		err     string
		setupFn func(event *errorEvent)
	}{
		{name: "traceID", err: "'parent_id' required",
			setupFn: func(e *errorEvent) { e.TraceID.Set("xxx") }},
		{name: "parentID", err: "'trace_id' required",
			setupFn: func(e *errorEvent) { e.ParentID.Set("xxx") }},
		{name: "transactionID", err: "'parent_id' required",
			setupFn: func(e *errorEvent) { e.TransactionID.Set("xxx") }},
		{name: "transactionID-parentID", err: "'trace_id' required",
			setupFn: func(e *errorEvent) {
				e.TransactionID.Set("xxx")
				e.ParentID.Set("xxx")
			}},
		{name: "transactionID-traceID", err: "'parent_id' required",
			setupFn: func(e *errorEvent) {
				e.TransactionID.Set("xxx")
				e.TraceID.Set("xxx")
			}},
	} {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			event := validErrorEvent()
			tc.setupFn(&event)
			err := event.validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.err)
		})
	}
}

func TestMetadataRequiredValidationRules(t *testing.T) {
	// setup: create full event struct with arbitrary values set
	var event metadata
	modeldecodertest.InitStructValues(&event)
	// test vanilla struct is valid
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"cloud.provider":          nil,
		"process.pid":             nil,
		"service":                 nil,
		"service.agent":           nil,
		"service.agent.name":      nil,
		"service.agent.version":   nil,
		"service.language.name":   nil,
		"service.runtime.name":    nil,
		"service.runtime.version": nil,
		"service.name":            nil,
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

func TestMetricsetRequiredValidationRules(t *testing.T) {
	// setup: create full struct with sample values set
	var root metricsetRoot
	s := `{"metricset":{"samples":{"a.b.":{"value":2048}},"timestamp":1496170421366000,"transaction":{"type":"request","name":"GET /"},"span":{"type":"db","subtype":"mysql"},"tags":{"a":"b"}}}`
	modeldecodertest.DecodeData(t, strings.NewReader(s), "metricset", &root)
	// test vanilla struct is valid
	event := root.Metricset
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"samples":       nil,
		"samples.value": nil,
	}
	modeldecodertest.SetZeroStructValue(&event, func(key string) {
		assertRequiredFn(t, requiredKeys, event.validate)
	})
}

func TestSpanRequiredValidationRules(t *testing.T) {
	// setup: create full struct with arbitrary values set
	var event span
	modeldecodertest.InitStructValues(&event)
	event.Outcome.Set("failure")
	// test vanilla struct is valid
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"id":                                   nil,
		"context.destination.service.name":     nil,
		"context.destination.service.resource": nil,
		"context.destination.service.type":     nil,
		"duration":                             nil,
		"name":                                 nil,
		"parent_id":                            nil,
		"trace_id":                             nil,
		"type":                                 nil,
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

func TestTransactionRequiredValidationRules(t *testing.T) {
	// setup: create full struct with arbitrary values set
	var event transaction
	modeldecodertest.InitStructValues(&event)
	event.Outcome.Set("success")
	// test vanilla struct is valid
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"duration":                  nil,
		"id":                        nil,
		"span_count":                nil,
		"span_count.started":        nil,
		"trace_id":                  nil,
		"type":                      nil,
		"context.request.method":    nil,
		"experience.longtask.count": nil,
		"experience.longtask.sum":   nil,
		"experience.longtask.max":   nil,
		"session.id":                nil,
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

var regexpArrayAccessor = regexp.MustCompile(`\[.*\]\.`)

func assertRequiredFn(t *testing.T, keys map[string]interface{}, validate func() error) func(key string) {
	return func(key string) {
		s := regexpArrayAccessor.ReplaceAllString(key, "")
		err := validate()
		if _, ok := keys[s]; ok {
			require.Error(t, err, key)
			for _, part := range strings.Split(s, ".") {
				assert.Contains(t, err.Error(), part)
			}
		} else {
			assert.NoError(t, err, key)
		}
	}
}

//
// Test Set() and Reset()
//

func TestResetIsSet(t *testing.T) {
	for name, root := range map[string]setter{
		"error":       &errorRoot{},
		"metadata":    &metadataRoot{},
		"metricset":   &metricsetRoot{},
		"span":        &spanRoot{},
		"transaction": &transactionRoot{},
	} {
		t.Run(name, func(t *testing.T) {
			r := testdataReader(t, testFileName(name))
			modeldecodertest.DecodeData(t, r, name, &root)
			require.True(t, root.IsSet())
			// call Reset and ensure initial state, except for array capacity
			root.Reset()
			assert.False(t, root.IsSet())
		})
	}
}

type setter interface {
	IsSet() bool
	Reset()
}

type validator interface {
	validate() error
}

type testcase struct {
	name     string
	errorKey string
	data     string
}

func testdataReader(t *testing.T, typ string) io.Reader {
	p := filepath.Join("..", "..", "..", "testdata", "intake-v2", fmt.Sprintf("%s.ndjson", typ))
	r, err := os.Open(p)
	require.NoError(t, err)
	return r
}

func testFileName(eventType string) string {
	if eventType == "metadata" {
		return eventType
	}
	return eventType + "s"
}

func testValidation(t *testing.T, eventType string, testcases []testcase, keys ...string) {
	for _, tc := range testcases {
		t.Run(tc.name+"/"+eventType, func(t *testing.T) {
			var event validator
			switch eventType {
			case "error":
				event = &errorEvent{}
			case "metadata":
				event = &metadata{}
			case "metricset":
				event = &metricset{}
			case "span":
				event = &span{}
			case "transaction":
				event = &transaction{}
			}
			r := testdataReader(t, testFileName(eventType))
			modeldecodertest.DecodeDataWithReplacement(t, r, eventType, tc.data, event, keys...)

			// run validation and checks
			err := event.validate()
			if tc.errorKey == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorKey)
			}
		})
	}
}
