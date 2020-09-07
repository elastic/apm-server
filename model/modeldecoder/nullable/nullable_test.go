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

package nullable

import (
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testType struct {
	S String    `json:"s"`
	I Int       `json:"i"`
	V Interface `json:"v"`
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func TestString(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         string
		isSet, fail bool
	}{
		{name: "values", input: `{"s":"agent-go"}`, val: "agent-go", isSet: true},
		{name: "empty", input: `{"s":""}`, isSet: true},
		{name: "null", input: `{"s":null}`},
		{name: "missing", input: `{}`},
		{name: "invalid", input: `{"s":1234}`, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.S.IsSet())
				assert.Equal(t, tc.val, testStruct.S.Val)
			}

			testStruct.S.Reset()
			assert.False(t, testStruct.S.IsSet())
			assert.Empty(t, testStruct.S.Val)

			testStruct.S.Set("teststring")
			assert.True(t, testStruct.S.IsSet())
			assert.Equal(t, "teststring", testStruct.S.Val)
		})
	}
}

func TestInt(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         int
		isSet, fail bool
	}{
		{name: "values", input: `{"i":44}`, val: 44, isSet: true},
		{name: "empty", input: `{"i":0}`, isSet: true},
		{name: "null", input: `{"i":null}`, isSet: false},
		{name: "missing", input: `{}`},
		{name: "invalid", input: `{"i":"1.0.1"}`, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.I.IsSet())
				assert.Equal(t, tc.val, testStruct.I.Val)
			}

			testStruct.I.Reset()
			assert.False(t, testStruct.I.IsSet())
			assert.Empty(t, testStruct.I.Val)

			testStruct.I.Set(55)
			assert.True(t, testStruct.I.IsSet())
			assert.Equal(t, 55, testStruct.I.Val)
		})
	}
}

func TestInterface(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         interface{}
		isSet, fail bool
	}{
		{name: "integer", input: `{"v":44}`, val: float64(44), isSet: true},
		{name: "string", input: `{"v":"1.0.1"}`, val: "1.0.1", isSet: true},
		{name: "bool", input: `{"v":true}`, val: true, isSet: true},
		{name: "empty", input: `{"v":""}`, val: "", isSet: true},
		{name: "null", input: `{"v":null}`},
		{name: "missing", input: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.V.IsSet())
				assert.Equal(t, tc.val, testStruct.V.Val)
			}

			testStruct.V.Reset()
			assert.False(t, testStruct.V.IsSet())
			assert.Empty(t, testStruct.V.Val)
		})
	}
}
