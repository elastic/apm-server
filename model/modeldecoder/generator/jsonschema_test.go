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

package generator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/elastic/apm-server/model/modeldecoder/generator/generatortest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

type testcase struct {
	n     string
	data  string
	valid bool
}

func nameOf(i interface{}) string {
	return reflect.TypeOf(i).Name()
}

func TestSchemaString(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.String{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "enum", data: `{"required":"closed"}`, valid: true},
		{n: "violation-enum", data: `{"required":"***"}`, valid: false},
		{n: "minLength", data: `{"required":"open","nullable":"12"}`, valid: true},
		{n: "violation-minLength", data: `{"required":"open","nullable":"1"}`, valid: false},
		{n: "maxLength", data: `{"required":"open","nullable":"12345"}`, valid: true},
		{n: "violation-maxLength", data: `{"required":"open","nullable":"123456"}`, valid: false},
		{n: "violation-pattern", data: `{"required":"open","nullable":"*.*"}`, valid: false},
		{n: "nullable", data: `{"required":"closed","nullable":null}`, valid: true},
		{n: "violation-required", data: `{"nullable":"***"}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":"closed","abc":"**"}`, valid: true},
		{n: "violation-type", data: `{"required":123}`, valid: false},
		{n: "nullable-enum", data: `{"required":"closed","nullable_enum":"closed"}`, valid: true},
		{n: "nullable-enum-null", data: `{"required":"closed","nullable_enum":null}`, valid: true},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaNumbers(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Number{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "int-min", data: `{"required":1}`, valid: true},
		{n: "int-max", data: `{"required":250}`, valid: true},
		{n: "violation-int-min", data: `{"required":0}`, valid: false},
		{n: "violation-int-max", data: `{"required":251}`, valid: false},
		{n: "float-min", data: `{"required":1,"nullable":0.5}`, valid: true},
		{n: "float-max", data: `{"required":1,"nullable":15.9}`, valid: true},
		{n: "violation-float-min", data: `{"required":1,"nullable":0.45}`, valid: false},
		{n: "violation-float-max", data: `{"required":1,"nullable":15.91}`, valid: false},
		{n: "nullable", data: `{"required":1,"nullable":null}`, valid: true},
		{n: "violation-required", data: `{"abc":123}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":1,"abc":123}`, valid: true},
		{n: "violation-type", data: `{"required":"123"}`, valid: false},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaBool(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Bool{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "valid", data: `{"required":true}`, valid: true},
		{n: "nullable", data: `{"required":false,"nullable":null}`, valid: true},
		{n: "violation-required", data: `{"nullable":true}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":true,"abc":123}`, valid: true},
		{n: "violation-type", data: `{"required":123}`, valid: false},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaHTTPHeader(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.HTTPHeader{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "http-header", data: `{"required":{"a":"v","b":["v1","v2"],"a":["v2"]}}`, valid: true},
		{n: "nullable", data: `{"required":{"a":"v"},"nullable":null}`, valid: true},
		{n: "violation-required", data: `{"nullable":{"a":"v"}}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":{"a":"v"},"abc":123}`, valid: true},
		{n: "violation-type-int", data: `{"required":123}`, valid: false},
		{n: "violation-type", data: `{"required":{"a":{"b":"v"}}}`, valid: false},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}
func TestSchemaInterface(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Interface{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "str-minLength", data: `{"required":"12"}`, valid: true},
		{n: "violation-str-minLength", data: `{"required":"1"}`, valid: false},
		{n: "str-maxLength", data: `{"required":"12345"}`, valid: true},
		{n: "violation-str-maxLength", data: `{"required":"123456"}`, valid: false},
		{n: "violation-str-pattern", data: `{"required":"12."}`, valid: false},
		{n: "int-min", data: `{"required":2}`, valid: true},
		{n: "int-max", data: `{"required":250}`, valid: true},
		{n: "violation-int-min", data: `{"required":0}`, valid: false},
		{n: "violation-int-max", data: `{"required":251}`, valid: false},
		{n: "float-min", data: `{"required":1.5}`, valid: true},
		{n: "float-max", data: `{"required":250.0}`, valid: true},
		{n: "violation-float-min", data: `{"required":0.45}`, valid: false},
		{n: "violation-float-max", data: `{"required":250.01}`, valid: false},
		{n: "bool", data: `{"required":false}`, valid: true},
		{n: "object", data: `{"required":{"a":{"b":123}}}`, valid: true},
		{n: "violation-type", data: `{"required":["a"]}`, valid: false},

		{n: "str-enum", data: `{"required":"12","nullable":"closed"}`, valid: true},
		{n: "violation-str-enum", data: `{"required":"12","nullable":"***"}`, valid: false},
		{n: "nullable", data: `{"required":2,"nullable":null}`, valid: true},
		{n: "violation-type", data: `{"required":2,"nullable":{"a":"b"}}`, valid: false},

		{n: "violation-required", data: `{"nullable":"abc"}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":"ab","abc":"**"}`, valid: true},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaMap(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Map{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "required", data: `{"required":{"aab":"abcde"}}`, valid: true},
		{n: "all-types", data: `{"required":{"aab":1,"abab":"abcde","bbb":true}}`, valid: true},
		{n: "violation-pattern", data: `{"required":{"tgh":"abcde"}}`, valid: false},
		{n: "violation-maxLengthVals", data: `{"required":{"aab":"abcdef"}}`, valid: false},
		{n: "violation-required", data: `{"nullable":{"aab":"abcde"}}`, valid: false},
		{n: "violation-required-null", data: `{"required":null}`, valid: false},
		{n: "additional-props", data: `{"required":{"aab":1},"additional":100}`, valid: true},
		{n: "violation-type", data: `{"required":{"aab":{}}}`, valid: false},
		{n: "nested-a", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{"required":10.9}}}`, valid: true},
		{n: "violation-nested-a-required", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{}}}`, valid: false},
		{n: "violation-nested-a-required-null", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{"required":null}}}`, valid: false},
		{n: "violation-nested-a-pattern", data: `{"required":{"aab":"abcde"},"nested_a":{"tt":{"required":10.9}}}`, valid: false},
		{n: "nested-a-nullable", data: `{"required":{"aab":"abcde"},"nested_a":null}`, valid: true},
		{n: "nested-b", data: `{"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2"}}}`, valid: true},
		{n: "nested-b-additional-props", data: `{"abc":123,"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2"}}}`, valid: true},
		{n: "failure-nested-b-additional-props-ab", data: `{"required":{"aab":"abcde"},"nested_b":{"xyz":{"a":"b"},"ab":{"c":"v1","cc":"v2"}}}`, valid: false},
		{n: "failure-nested-b-additional-props-c", data: `{"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2","d":2}}}`, valid: false},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaSliceStruct(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Slice{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "required", data: `{"required":["cc","cc","ccc","cc"]}`, valid: true},
		{n: "violation-mexLength", data: `{"required":["cccc"]}`, valid: false},
		{n: "violation-minLength", data: `{"required":["c"]}`, valid: false},
		{n: "violation-minElems", data: `{"required":[]}`, valid: false},
		{n: "violation-required-null", data: `{"required":[null]}`, valid: false},
		{n: "valid", data: `{"required":["cc"],"nullable":["b"]}`, valid: true},
		{n: "violation-pattern", data: `{"required":["cc"],"nullable":["cde"]}`, valid: false},
		{n: "nullable-minElems", data: `{"required":["cc"],"nullable":[]}`, valid: true},
		{n: "violation-nullable-null", data: `{"required":["cc"],"nullable":[null]}`, valid: false},
		{n: "struct-minElems", data: `{"required":["cc"],"children":[]}`, valid: true},
		{n: "struct-null", data: `{"required":["cc"],"children":[{}]}`, valid: true},
		{n: "struct-recursive", data: `{"required":["cc"],"children":[{"number":2.3,"children":[{"number":2.3,"children":[{"number":2.3}]}]}]}`, valid: true},
		{n: "struct-additional-props", data: `{"required":["cc"],"abc":2,"children":[{"a":1}]}`, valid: true},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaRequiredIfAny(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.RequiredIfAny{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "all", data: `{"a":"a","b":"b","c":"c"}`, valid: true},
		{n: "none", data: `{}`, valid: true},
		{n: "only-a", data: `{"a":"a"}`, valid: true},
		{n: "no-b", data: `{"a":"a","c":"c"}`, valid: true},
		{n: "violation-a-if-b", data: `{"b":"b","c":"c"}`, valid: false},
		{n: "violation-a-if-c", data: `{"c":"c"}`, valid: false},
		{n: "violation-c-if-b", data: `{"a":"a","b":"b"}`, valid: false},
		{n: "additional-props", data: `{"xyc":34}`, valid: true},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaRequiredAnyOf(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.RequiredAnyOf{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "all", data: `{"a":1,"b":2}`, valid: true},
		{n: "a", data: `{"a":1}`, valid: true},
		{n: "b", data: `{"b":1}`, valid: true},
		{n: "violation-required", data: `{}`, valid: false},
		{n: "additional-props", data: `{"a":1,"xyc":34}`, valid: true},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func TestSchemaOnlyExported(t *testing.T) {
	schema, err := generateJSONSchema(nameOf(generatortest.Exported{}))
	require.NoError(t, err)
	for _, tc := range []testcase{
		{n: "valid", data: `{"b":1}`, valid: true},
		{n: "unexported-no-type-checking", data: `{"a":1.5,"_":0.7}`, valid: true},
		{n: "violation-number", data: `{"b":1.5}`, valid: false},
	} {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.valid, result.Valid(), result.Errors())
		})
	}
}

func validate(schema string, data string) (*gojsonschema.Result, error) {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	dataLoader := gojsonschema.NewStringLoader(data)
	return gojsonschema.Validate(schemaLoader, dataLoader)
}

func generateJSONSchema(root string) (string, error) {
	var s string
	p := filepath.Join("github.com", "elastic", "apm-server", "model", "modeldecoder", "generator", "generatortest")
	parsed, err := Parse(p)
	if err != nil {
		return s, err
	}
	jsonSchema, err := NewJSONSchemaGenerator(parsed)
	if err != nil {
		return s, err
	}
	rootEvent := fmt.Sprintf("%s.%s", p, root)
	b, err := jsonSchema.Generate("jsonschematest", rootEvent)
	return b.String(), err
}
