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
		{name: "id-float", errorKey: "types", data: `{"id":45.6}`},
		{name: "id-bool", errorKey: "types", data: `{"id":true}`},
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
		{name: "service-name-invalid", errorKey: "regexpAlphaNumericExt",
			data: `{"agent":{"name":"go","version":"1.0"},"name":"⌘"}`},
		{name: "service-name-max", data: `{"agent":{"name":"go","version":"1.0"},"name":"` + modeldecodertest.BuildStringWith(1024, '-') + `"}`},
		{name: "service-name-max-exceeded", errorKey: "max",
			data: `{"agent":{"name":"go","version":"1.0"},"name":"` + modeldecodertest.BuildStringWith(1025, '-') + `"}`},
	}
	testValidation(t, "metadata", testcases, "service")
	testValidation(t, "transaction", testcases, "context", "service")
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
	testValidation(t, "metadata", testcases, "labels")
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
			{name: "custom", data: `{"custom":{"k1":{"v1":123,"v2":"value"},"k2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
			{name: "custom-key-dot", errorKey: "patternKeys", data: `{"custom":{"k1.":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-asterisk", errorKey: "patternKeys", data: `{"custom":{"k1*":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-quote", errorKey: "patternKeys", data: `{"custom":{"k1\"":{"v1":123,"v2":"value"}}}`},
		}
		testValidation(t, "transaction", testcases, "context")
	})

	t.Run("tags", func(t *testing.T) {
		testcases := []testcase{
			{name: "tags", data: `{"tags":{"k1":"v1.s*\"","k2":34,"k3":23.56,"k4":true}}`},
			{name: "tags-key-dot", errorKey: "patternKeys", data: `{"tags":{"k1.":"v1"}}`},
			{name: "tags-key-asterisk", errorKey: "patternKeys", data: `{"tags":{"k1*":"v1"}}`},
			{name: "tags-key-quote", errorKey: "patternKeys", data: `{"tags":{"k1\"":"v1"}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":"abc"}}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":[1,2,3]}}}`},
			{name: "tags-maxVal", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "tags-maxVal-exceeded", errorKey: "maxVals", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1025) + `"}}`},
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
		{name: "marks", data: `{"k1":{"v1":12.3}}`},
		{name: "marks-dot", errorKey: "patternKeys", data: `{"k.1":{"v1":12.3}}`},
		{name: "marks-events-dot", errorKey: "patternKeys", data: `{"k1":{"v.1":12.3}}`},
		{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k*1":{"v1":12.3}}`},
		{name: "marks-events-asterisk", errorKey: "patternKeys", data: `{"k1":{"v*1":12.3}}`},
		{name: "marks-quote", errorKey: "patternKeys", data: `{"k\"1":{"v1":12.3}}`},
		{name: "marks-events-quote", errorKey: "patternKeys", data: `{"k1":{"v\"1":12.3}}`},
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
}

func TestURLValidationRules(t *testing.T) {
	testcases := []testcase{
		{name: "port-string", data: `{"request":{"method":"get","url":{"port":"8200"}}}`},
		{name: "port-int", data: `{"request":{"method":"get","url":{"port":8200}}}`},
		{name: "port-invalid-type", errorKey: "types",
			data: `{"request":{"method":"get","url":{"port":[8200,8201]}}}`},
		{name: "port-invalid-type", errorKey: "types",
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
		require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(&out))
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
}

//
// Test Required fields
//

func TestMetadataRequiredValidationRules(t *testing.T) {
	// setup: create full metadata struct with arbitrary values set
	var metadata metadata
	modeldecodertest.InitStructValues(&metadata)
	// test vanilla struct is valid
	require.NoError(t, metadata.validate())

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
	// setup: create full struct with arbitrary values set
	var event transaction
	modeldecodertest.InitStructValues(&event)
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

//
// Test Set() and Reset()
//

func TestResetIsSet(t *testing.T) {
	for name, root := range map[string]setter{
		"metadata":    &metadataRoot{},
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
			case "metadata":
				event = &metadata{}
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
