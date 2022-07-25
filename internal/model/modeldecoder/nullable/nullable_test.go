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
	"net/http"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testType struct {
	S   String         `json:"s"`
	I   Int            `json:"i"`
	F   Float64        `json:"f"`
	B   Bool           `json:"b"`
	V   Interface      `json:"v"`
	Tms TimeMicrosUnix `json:"tms"`
	H   HTTPHeader     `json:"h"`
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

func TestFloat64(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         float64
		isSet, fail bool
	}{
		{name: "values", input: `{"f":44.89}`, val: 44.89, isSet: true},
		{name: "integer", input: `{"f":44}`, val: 44.00, isSet: true},
		{name: "zero", input: `{"f":0}`, isSet: true},
		{name: "null", input: `{"f":null}`, isSet: false},
		{name: "missing", input: `{}`},
		{name: "invalid", input: `{"f":"1.0.1"}`, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.F.IsSet())
				assert.Equal(t, tc.val, testStruct.F.Val)
			}

			testStruct.F.Reset()
			assert.False(t, testStruct.F.IsSet())
			assert.Empty(t, testStruct.F.Val)

			testStruct.F.Set(55.67)
			assert.True(t, testStruct.F.IsSet())
			assert.Equal(t, 55.67, testStruct.F.Val)
		})
	}
}

func TestBool(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         bool
		isSet, fail bool
	}{
		{name: "true", input: `{"b":true}`, val: true, isSet: true},
		{name: "false", input: `{"b":false}`, val: false, isSet: true},
		{name: "null", input: `{"b":null}`, isSet: false},
		{name: "missing", input: `{}`},
		{name: "convert", input: `{"b":1}`, fail: true},
		{name: "invalid", input: `{"b":"1.0.1"}`, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.B.IsSet())
				assert.Equal(t, tc.val, testStruct.B.Val)
			}

			testStruct.B.Reset()
			assert.False(t, testStruct.B.IsSet())
			assert.Empty(t, testStruct.B.Val)

			testStruct.B.Set(true)
			assert.True(t, testStruct.B.IsSet())
			assert.Equal(t, true, testStruct.B.Val)
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

func TestTimeMicrosUnix(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         string
		isSet, fail bool
	}{
		{name: "valid", input: `{"tms":1599996822281000}`, isSet: true,
			val: "2020-09-13 11:33:42.281 +0000 UTC"},
		{name: "null", input: `{"tms":null}`, val: time.Time{}.String()},
		{name: "invalid-type", input: `{"tms":""}`, fail: true, isSet: true},
		{name: "invalid-type", input: `{"tms":123.56}`, fail: true, isSet: true},
		{name: "missing", input: `{}`, val: time.Time{}.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tc.input))
			var testStruct testType
			err := dec.Decode(&testStruct)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.isSet, testStruct.Tms.IsSet())
				assert.Equal(t, tc.val, testStruct.Tms.Val.String())
			}

			testStruct.Tms.Reset()
			assert.False(t, testStruct.Tms.IsSet())
			assert.Zero(t, testStruct.Tms.Val)

			testStruct.Tms.Set(time.Now())
			assert.True(t, testStruct.Tms.IsSet())
			assert.NotZero(t, testStruct.Tms.Val)
		})
	}
}
func TestHTTPHeader(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string

		val         http.Header
		isSet, fail bool
	}{
		{name: "valid", isSet: true, input: `
{"h":{"content-type":"application/x-ndjson","Authorization":"Bearer 123-token","authorization":"ApiKey 123-api-key","Accept":["text/html", "application/xhtml+xml"]}}`,
			val: http.Header{
				"Content-Type":  []string{"application/x-ndjson"},
				"Authorization": []string{"ApiKey 123-api-key", "Bearer 123-token"},
				"Accept":        []string{"text/html", "application/xhtml+xml"},
			}},
		{name: "valid2", input: `{"h":{"k":["a","b"]}}`, isSet: true, val: http.Header{"K": []string{"a", "b"}}},
		{name: "null", input: `{"h":null}`},
		{name: "invalid-type", input: `{"h":""}`, fail: true, isSet: true},
		{name: "invalid-array", input: `{"h":{"k":["a",23]}}`, isSet: true, fail: true},
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
				assert.Equal(t, tc.isSet, testStruct.H.IsSet())
				assert.Equal(t, len(tc.val), len(testStruct.H.Val))
				for k, v := range tc.val {
					assert.ElementsMatch(t, v, testStruct.H.Val.Values(k))
				}
			}

			testStruct.H.Reset()
			assert.False(t, testStruct.H.IsSet())
			assert.Empty(t, testStruct.H.Val)

			testStruct.H.Set(http.Header{"Accept": []string{"*/*"}})
			assert.True(t, testStruct.H.IsSet())
			assert.NotEmpty(t, testStruct.H.Val)
		})
	}
}
