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
}

func TestLabelValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "valid", data: `{"k1":"v1","k2":2.3,"k3":3,"k4":true,"k5":null}`},
		{name: "restricted-type", errorKey: "typesVals", data: `{"k1":{"k2":"v1"}}`},
		{name: "key-dot", errorKey: "patternKeys", data: `{"k.1":"v1"}`},
		{name: "key-asterisk", errorKey: "patternKeys", data: `{"k*1":"v1"}`},
		{name: "key-quotemark", errorKey: "patternKeys", data: `{"k\"1":"v1"}`},
		{name: "max-len", data: `{"k1":"` + modeldecodertest.BuildString(1024) + `"}`},
		{name: "max-len-exceeded", errorKey: "maxVals", data: `{"k1":"` + modeldecodertest.BuildString(1025) + `"}`},
	}
	testValidation(t, "m", testcases, "l")
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
	})

	t.Run("tags", func(t *testing.T) {
		testcases := []testcase{
			{name: "tags", data: `{"g":{"k1":"v1.s*\"","k2":34,"k3":23.56,"k4":true}}`},
			{name: "tags-key-dot", errorKey: "patternKeys", data: `{"g":{"k1.":"v1"}}`},
			{name: "tags-key-asterisk", errorKey: "patternKeys", data: `{"g":{"k1*":"v1"}}`},
			{name: "tags-key-quote", errorKey: "patternKeys", data: `{"g":{"k1\"":"v1"}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"g":{"k1":{"v1":"abc"}}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"g":{"k1":{"v1":[1,2,3]}}}`},
			{name: "tags-maxVal", data: `{"g":{"k1":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "tags-maxVal-exceeded", errorKey: "maxVals", data: `{"g":{"k1":"` + modeldecodertest.BuildString(1025) + `"}}`},
		}
		testValidation(t, "x", testcases, "c")
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

func TestMetadataRequiredValidationRules(t *testing.T) {
	// setup: create full metadata struct with arbitrary values set
	var metadata metadata
	modeldecodertest.InitStructValues(&metadata)

	// test vanilla struct is valid
	require.NoError(t, metadata.validate())

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
	modeldecodertest.SetZeroStructValue(&metadata, func(key string) {
		err := metadata.validate()
		if _, ok := requiredKeys[key]; ok {
			require.Error(t, err, key)
			for _, part := range strings.Split(key, ".") {
				assert.Contains(t, err.Error(), part)
			}
		} else {
			assert.NoError(t, err, key)
		}
	})
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
	requiredKeys := map[string]interface{}{"d": nil,
		"id":           nil,
		"yc":           nil,
		"yc.sd":        nil,
		"tid":          nil,
		"t":            nil,
		"c.q.mt":       nil,
		"exp.lt.count": nil,
		"exp.lt.sum":   nil,
		"exp.lt.max":   nil,
	}
	modeldecodertest.SetZeroStructValue(&event, func(key string) {
		err := event.validate()
		if _, ok := requiredKeys[key]; ok {
			require.Error(t, err, key)
			for _, part := range strings.Split(key, ".") {
				assert.Contains(t, err.Error(), part)
			}
		} else {
			assert.NoError(t, err, key)
		}
	})
}

func TestResetIsSet(t *testing.T) {
	for name, root := range map[string]setter{
		"m": &metadataRoot{},
		"x": &transactionRoot{},
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
	case "me":
		return eventType
	default:
		return "rum_events"
	}
}

func testValidation(t *testing.T, eventType string, testcases []testcase, keys ...string) {
	for _, tc := range testcases {
		t.Run(tc.name+"/"+eventType, func(t *testing.T) {
			var event validator
			switch eventType {
			case "m":
				event = &metadata{}
			case "x":
				event = &transaction{}
				// case "y":
				// 	event = &span{}
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
