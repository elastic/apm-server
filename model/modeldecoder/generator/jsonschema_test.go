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
	"path"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"

	"github.com/elastic/apm-server/model/modeldecoder/generator/testdata"
)

type testcase struct {
	n    string
	data string
}

func nameOf(i interface{}) string {
	return reflect.TypeOf(i).Name()
}

func TestSchemaString(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.String{}))
	assertValid(t, schema, []testcase{
		{n: "enum", data: `{"required":"closed"}`},
		{n: "minLength", data: `{"required":"open","nullable":"12"}`},
		{n: "maxLength", data: `{"required":"open","nullable":"12345"}`},
		{n: "nullable", data: `{"required":"closed","nullable":null}`},
		{n: "additional-props", data: `{"required":"closed","abc":"**"}`},
		{n: "nullable-enum", data: `{"required":"closed","nullable_enum":"closed"}`},
		{n: "nullable-enum-null", data: `{"required":"closed","nullable_enum":null}`}})
	assertInvalid(t, schema, []testcase{
		{n: "enum", data: `{"required":"***"}`},
		{n: "minLength", data: `{"required":"open","nullable":"1"}`},
		{n: "maxLength", data: `{"required":"open","nullable":"123456"}`},
		{n: "pattern", data: `{"required":"open","nullable":"*.*"}`},
		{n: "required", data: `{"nullable":"***"}`},
		{n: "required-null", data: `{"required":null}`},
		{n: "type", data: `{"required":123}`},
	})
}

func TestSchemaNumbers(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Number{}))
	assertValid(t, schema, []testcase{
		{n: "int-min", data: `{"required":1}`},
		{n: "int-max", data: `{"required":250}`},
		{n: "float-min", data: `{"required":1,"nullable":0.5}`},
		{n: "float-max", data: `{"required":1,"nullable":15.9}`},
		{n: "nullable", data: `{"required":1,"nullable":null}`},
		{n: "additional-props", data: `{"required":1,"abc":123}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-int-min", data: `{"required":0}`},
		{n: "violation-int-max", data: `{"required":251}`},
		{n: "violation-float-min", data: `{"required":1,"nullable":0.45}`},
		{n: "violation-float-max", data: `{"required":1,"nullable":15.91}`},
		{n: "violation-required", data: `{"abc":123}`},
		{n: "violation-required-null", data: `{"required":null}`},
		{n: "violation-type", data: `{"required":"123"}`}})
}

func TestSchemaBool(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Bool{}))
	assertValid(t, schema, []testcase{
		{n: "valid", data: `{"required":true}`},
		{n: "nullable", data: `{"required":false,"nullable":null}`},
		{n: "additional-props", data: `{"required":true,"abc":123}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-required", data: `{"nullable":true}`},
		{n: "violation-required-null", data: `{"required":null}`},
		{n: "violation-type", data: `{"required":123}`}})
}

func TestSchemaHTTPHeader(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.HTTPHeader{}))
	assertValid(t, schema, []testcase{
		{n: "http-header", data: `{"required":{"a":"v","b":["v1","v2"],"a":["v2"]}}`},
		{n: "nullable", data: `{"required":{"a":"v"},"nullable":null}`},
		{n: "additional-props", data: `{"required":{"a":"v"},"abc":123}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-required", data: `{"nullable":{"a":"v"}}`},
		{n: "violation-required-null", data: `{"required":null}`},
		{n: "violation-type-int", data: `{"required":123}`},
		{n: "violation-type", data: `{"required":{"a":{"b":"v"}}}`}})
}
func TestSchemaInterface(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Interface{}))
	assertValid(t, schema, []testcase{
		{n: "str-minLength", data: `{"required":"12"}`},
		{n: "str-maxLength", data: `{"required":"12345"}`},
		{n: "int-min", data: `{"required":2}`},
		{n: "int-max", data: `{"required":250}`},
		{n: "float-min", data: `{"required":1.5}`},
		{n: "float-max", data: `{"required":250.0}`},
		{n: "bool", data: `{"required":false}`},
		{n: "object", data: `{"required":{"a":{"b":123}}}`},
		{n: "str-enum", data: `{"required":"12","nullable":"closed"}`},
		{n: "nullable", data: `{"required":2,"nullable":null}`},
		{n: "additional-props", data: `{"required":"ab","abc":"**"}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-str-minLength", data: `{"required":"1"}`},
		{n: "violation-str-maxLength", data: `{"required":"123456"}`},
		{n: "violation-str-pattern", data: `{"required":"12."}`},
		{n: "violation-int-min", data: `{"required":0}`},
		{n: "violation-int-max", data: `{"required":251}`},
		{n: "violation-float-min", data: `{"required":0.45}`},
		{n: "violation-float-max", data: `{"required":250.01}`},
		{n: "violation-type", data: `{"required":["a"]}`},
		{n: "violation-str-enum", data: `{"required":"12","nullable":"***"}`},
		{n: "violation-type", data: `{"required":2,"nullable":{"a":"b"}}`},
		{n: "violation-required", data: `{"nullable":"abc"}`},
		{n: "violation-required-null", data: `{"required":null}`}})
}

func TestSchemaMap(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Map{}))
	assertValid(t, schema, []testcase{
		{n: "required", data: `{"required":{"aab":"abcde"}}`},
		{n: "all-types", data: `{"required":{"aab":1,"abab":"abcde","bbb":true}}`},
		{n: "additional-props", data: `{"required":{"aab":1},"additional":100}`},
		{n: "nested-a", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{"required":10.9}}}`},
		{n: "nested-a-nullable", data: `{"required":{"aab":"abcde"},"nested_a":null}`},
		{n: "nested-b", data: `{"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2"}}}`},
		{n: "nested-b-additional-props", data: `{"abc":123,"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2"}}}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-pattern", data: `{"required":{"tgh":"abcde"}}`},
		{n: "violation-maxLengthVals", data: `{"required":{"aab":"abcdef"}}`},
		{n: "violation-required", data: `{"nullable":{"aab":"abcde"}}`},
		{n: "violation-required-null", data: `{"required":null}`},
		{n: "violation-type", data: `{"required":{"aab":{}}}`},
		{n: "violation-nested-a-required", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{}}}`},
		{n: "violation-nested-a-required-null", data: `{"required":{"aab":"abcde"},"nested_a":{"aaa":{"required":null}}}`},
		{n: "violation-nested-a-pattern", data: `{"required":{"aab":"abcde"},"nested_a":{"tt":{"required":10.9}}}`},
		{n: "failure-nested-b-additional-props-ab", data: `{"required":{"aab":"abcde"},"nested_b":{"xyz":{"a":"b"},"ab":{"c":"v1","cc":"v2"}}}`},
		{n: "failure-nested-b-additional-props-c", data: `{"required":{"aab":"abcde"},"nested_b":{"ab":{"c":"v1","cc":"v2","d":2}}}`}})
}

func TestSchemaSliceStruct(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Slice{}))
	assertValid(t, schema, []testcase{
		{n: "required", data: `{"required":["cc","cc","ccc","cc"]}`},
		{n: "valid", data: `{"required":["cc"],"nullable":["b"]}`},
		{n: "nullable-minElems", data: `{"required":["cc"],"nullable":[]}`},
		{n: "struct-minElems", data: `{"required":["cc"],"children":[]}`},
		{n: "struct-null", data: `{"required":["cc"],"children":[{}]}`},
		{n: "struct-recursive", data: `{"required":["cc"],"children":[{"number":2.3,"children":[{"number":2.3,"children":[{"number":2.3}]}]}]}`},
		{n: "struct-additional-props", data: `{"required":["cc"],"abc":2,"children":[{"a":1}]}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-mexLength", data: `{"required":["cccc"]}`},
		{n: "violation-minLength", data: `{"required":["c"]}`},
		{n: "violation-minElems", data: `{"required":[]}`},
		{n: "violation-required-null", data: `{"required":[null]}`},
		{n: "violation-pattern", data: `{"required":["cc"],"nullable":["cde"]}`},
		{n: "violation-nullable-null", data: `{"required":["cc"],"nullable":[null]}`}})
}

func TestSchemaRequiredIfAny(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.RequiredIfAny{}))
	assertValid(t, schema, []testcase{
		{n: "all", data: `{"a":"a","b":"b","c":"c","d":"d"}`},
		{n: "only-required", data: `{"d":"d"}`},
		{n: "only-a", data: `{"a":"a","d":"d"}`},
		{n: "no-b", data: `{"a":"a","c":"c","d":"d"}`},
		{n: "additional-props", data: `{"xyc":34,"d":"d"}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-none", data: `{}`},
		{n: "violation-a-if-b", data: `{"b":"b","c":"c","d":"d"}`},
		{n: "violation-a-if-c", data: `{"c":"c","d":"d"}`},
		{n: "violation-c-if-b", data: `{"a":"a","b":"b","d":"d"}`}})
}

func TestSchemaRequiredAnyOf(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.RequiredAnyOf{}))
	assertValid(t, schema, []testcase{
		{n: "all", data: `{"a":1,"b":2}`},
		{n: "a", data: `{"a":1}`},
		{n: "b", data: `{"b":1}`},
		{n: "additional-props", data: `{"a":1,"xyc":34}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-required", data: `{}`}})
}

func TestSchemaOnlyExported(t *testing.T) {
	schema := generateJSONSchema(t, nameOf(testdata.Exported{}))
	assertValid(t, schema, []testcase{
		{n: "valid", data: `{"b":1}`},
		{n: "unexported-no-type-checking", data: `{"a":1.5,"_":0.7}`}})
	assertInvalid(t, schema, []testcase{
		{n: "violation-number", data: `{"b":1.5}`}})
}

func validate(schema string, data string) (*gojsonschema.Result, error) {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	dataLoader := gojsonschema.NewStringLoader(data)
	return gojsonschema.Validate(schemaLoader, dataLoader)
}

func generateJSONSchema(t *testing.T, root string) string {
	p := path.Join("github.com", "elastic", "apm-server", "model", "modeldecoder", "generator", "testdata")
	parsed, err := Parse(p)
	require.NoError(t, err, err)
	jsonSchema, err := NewJSONSchemaGenerator(parsed)
	require.NoError(t, err, err)
	rootEvent := fmt.Sprintf("%s.%s", p, root)
	b, err := jsonSchema.Generate("jsonschematest", rootEvent)
	require.NoError(t, err, err)
	return b.String()
}

func assertValid(t *testing.T, schema string, cases []testcase) {
	for _, tc := range cases {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.True(t, result.Valid(), result.Errors())
		})
	}
}

func assertInvalid(t *testing.T, schema string, cases []testcase) {
	for _, tc := range cases {
		t.Run(tc.n, func(t *testing.T) {
			result, err := validate(schema, tc.data)
			require.NoError(t, err)
			assert.False(t, result.Valid(), result.Errors())
		})
	}
}
