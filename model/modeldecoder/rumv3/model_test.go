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

package rumv3

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

	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestUserValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "id-string", data: `{"id":"user123"}`},
		{name: "id-int", data: `{"id":44}`},
		{name: "id-float", errorKey: "types", data: `{"id":45.6}`},
		{name: "id-bool", errorKey: "types", data: `{"id":true}`},
		{name: "id-string-max-len", data: `{"id":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "id-string-max-len", errorKey: "max", data: `{"id":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "m", testcases, "u")
	testValidation(t, "x", testcases, "c", "u")
	testValidation(t, "e", testcases, "c", "u")
}

func TestServiceValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "name-valid-lower", data: `{"a":{"n":"go","ve":"1.0"},"n":"abcdefghijklmnopqrstuvwxyz"}`},
		{name: "name-valid-upper", data: `{"a":{"n":"go","ve":"1.0"},"n":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"}`},
		{name: "name-valid-digits", data: `{"a":{"n":"go","ve":"1.0"},"n":"0123456789"}`},
		{name: "name-valid-special", data: `{"a":{"n":"go","ve":"1.0"},"n":"_ -"}`},
		{name: "name-asterisk", errorKey: "n", data: `{"a":{"n":"go","ve":"1.0"},"n":"abc*"}`},
		{name: "name-dot", errorKey: "n", data: `{"a":{"n":"go","ve":"1.0"},"n":"abc."}`},
	}
	testValidation(t, "m", testcases, "se")
	testValidation(t, "x", testcases, "c", "se")
	testValidation(t, "e", testcases, "c", "se")
}

func TestLabelValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "valid", data: `{"k1":"v1.s*\"","k2":2.3,"k3":3,"k4":true,"k5":null}`},
		{name: "restricted-type", errorKey: "typesVals", data: `{"k1":{"k2":"v1"}}`},
		{name: "restricted-type", errorKey: "typesVals", data: `{"k1":{"k2":[1,2,3]}}`},
		{name: "key-dot", errorKey: "patternKeys", data: `{"k.1":"v1"}`},
		{name: "key-asterisk", errorKey: "patternKeys", data: `{"k*1":"v1"}`},
		{name: "key-quotemark", errorKey: "patternKeys", data: `{"k\"1":"v1"}`},
		{name: "max-len", data: `{"k1":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "max-len-exceeded", errorKey: "maxVals", data: `{"k1":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "m", testcases, "l")
	testValidation(t, "x", testcases, "c", "g")
	testValidation(t, "e", testcases, "c", "g")
}

func TestMaxLenValidationRules(t *testing.T) {
	// this tests an arbitrary field to ensure the `max` rule on strings works as expected
	testcases := []testcase{
		{name: "service-environment-max-len",
			data: `{"a":{"n":"go","ve":"1.0"},"n":"my-service","en":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "service-environment-max-len", errorKey: "max",
			data: `{"a":{"n":"go","ve":"1.0"},"n":"my-service","en":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "m", testcases, "se")
}

func TestContextValidationRules(t *testing.T) {
	t.Run("custom", func(t *testing.T) {
		testcases := []testcase{
			{name: "custom", data: `{"cu":{"k1":{"v1":123,"v2":"value"},"k2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
			{name: "custom-key-dot", errorKey: "patternKeys", data: `{"cu":{"k1.":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-asterisk", errorKey: "patternKeys", data: `{"cu":{"k1*":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-quote", errorKey: "patternKeys", data: `{"cu":{"k1\"":{"v1":123,"v2":"value"}}}`},
		}
		testValidation(t, "x", testcases, "c")
		testValidation(t, "e", testcases, "c")
	})
}

func TestDurationValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "duration", data: `0.0`},
		{name: "duration", errorKey: "min", data: `-0.09`},
	}
	testValidation(t, "x", testcases, "d")
}

func TestMarksValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "marks", data: `{"k1":{"v1":12.3}}`},
		{name: "marks-dot", errorKey: "patternKeys", data: `{"k.1":{"v1":12.3}}`},
		{name: "marks-event-dot", errorKey: "patternKeys", data: `{"k1":{"v.1":12.3}}`},
		{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k*1":{"v1":12.3}}`},
		{name: "marks-event-asterisk", errorKey: "patternKeys", data: `{"k1":{"v*1":12.3}}`},
		{name: "marks-quote", errorKey: "patternKeys", data: `{"k\"1":{"v1":12.3}}`},
		{name: "marks-event-quote", errorKey: "patternKeys", data: `{"k1":{"v\"1":12.3}}`},
	}
	testValidation(t, "x", testcases, "k")
}

func TestOutcomeValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "outcome-success", data: `"success"`},
		{name: "outcome-failure", data: `"failure"`},
		{name: "outcome-unknown", data: `"unknown"`},
		{name: "outcome-invalid", errorKey: "enum", data: `"anything"`},
	}
	testValidation(t, "x", testcases, "o")
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
		"c.q.mt":   nil, //context.request.method
		"ex.st.f":  nil, //log.stacktrace.filename
		"id":       nil, //id
		"log.mg":   nil, //log.message
		"log.st.f": nil, //log.stacktrace.filename
		"pid":      nil, //requiredIf
		"tid":      nil, //requiredIf
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
			err:     "requires at least one of the fields 'ex;log'",
			setupFn: func(e *errorEvent) {}},
		{name: "exception",
			err: "ex: requires at least one of the fields 'mg;t'",
			setupFn: func(e *errorEvent) {
				exception := errorException{}
				exception.Handled.Set(true)
				e.Exception = exception
			},
		},
		{name: "exception/cause",
			err: "ex: ca: requires at least one of the fields 'mg;t'",
			setupFn: func(e *errorEvent) {
				exception := errorException{}
				exception.Type.Set("test type")
				cause := errorException{}
				cause.Code.Set("400")
				exception.Cause = []errorException{cause}
				e.Exception = exception
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
		{name: "traceID", err: "'pid' required",
			setupFn: func(e *errorEvent) { e.TraceID.Set("xxx") }},
		{name: "parentID", err: "'tid' required",
			setupFn: func(e *errorEvent) { e.ParentID.Set("xxx") }},
		{name: "transactionID", err: "'pid' required",
			setupFn: func(e *errorEvent) { e.TransactionID.Set("xxx") }},
		{name: "transactionID-parentID", err: "'tid' required",
			setupFn: func(e *errorEvent) {
				e.TransactionID.Set("xxx")
				e.ParentID.Set("xxx")
			}},
		{name: "transactionID-traceID", err: "'pid' required",
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
		"se":       nil, //service
		"se.a":     nil, //service.agent
		"se.a.n":   nil, //service.agent.name
		"se.a.ve":  nil, //service.agent.version
		"se.la.n":  nil, //service.language.name
		"se.ru.n":  nil, //service.runtime.name
		"se.ru.ve": nil, //service.runtime.version
		"se.n":     nil, //service.name
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

func TestMetricsetRequiredValidationRules(t *testing.T) {
	// setup: create full struct with sample values set
	var root metricsetRoot
	s := `{"me":{"sa":{"xds":{"v":2048},"xbc":{"v":1}},"y":{"t":"db","su":"mysql"},"g":{"a":"b"}}}`
	modeldecodertest.DecodeData(t, strings.NewReader(s), "me", &root)
	// test vanilla struct is valid
	event := root.Metricset
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"sa":   nil, //samples
		"sa.v": nil, //samples.*.value
	}
	cb := assertRequiredFn(t, requiredKeys, event.validate)
	modeldecodertest.SetZeroStructValue(&event, cb)
}

func TestTransactionRequiredValidationRules(t *testing.T) {
	// setup: create full metadata struct with arbitrary values set
	var event transaction
	modeldecodertest.InitStructValues(&event)
	event.Outcome.Set("success")
	// test vanilla struct is valid
	require.NoError(t, event.validate())

	// iterate through struct, remove every key one by one
	// and test that validation behaves as expected
	requiredKeys := map[string]interface{}{
		"c.q.mt":       nil, //context.request.method
		"d":            nil, //duration
		"id":           nil, //id
		"exp.lt.count": nil, //user_experience.longtask.count
		"exp.lt.max":   nil, //user_experience.longtask.max
		"exp.lt.sum":   nil, //user_experience.longtask.sum
		"me.sa":        nil, //metricset.samples
		"t":            nil, //type
		"tid":          nil, //trace_id
		"yc":           nil, //span_count
		"yc.sd":        nil, //span_count.started
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
		"e":  &errorRoot{},
		"m":  &metadataRoot{},
		"me": &metricsetRoot{},
		"x":  &transactionRoot{},
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

type testcase struct {
	name     string
	errorKey string
	data     string
}

type setter interface {
	IsSet() bool
	Reset()
}

type validator interface {
	validate() error
}

func testdataReader(t *testing.T, typ string) io.Reader {
	p := filepath.Join("..", "..", "..", "testdata", "intake-v3", fmt.Sprintf("%s.ndjson", typ))
	r, err := os.Open(p)
	require.NoError(t, err)
	return r
}

func testFileName(eventType string) string {
	switch eventType {
	case "e":
		return "rum_errors"
	default:
		return "rum_events"
	}
}

func testValidation(t *testing.T, eventType string, testcases []testcase, keys ...string) {
	for _, tc := range testcases {
		t.Run(tc.name+"/"+eventType, func(t *testing.T) {
			var event validator
			switch eventType {
			case "e":
				event = &errorEvent{}
			case "m":
				event = &metadata{}
			case "me":
				event = &metricset{}
			case "x":
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
