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

package utility

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	str, str2                                   = "foo", "bar"
	integer, integer2, intFl32, intFl64         = 12.0, 24.0, 32, 64
	fl64, fl64Also                      float64 = 7.1, 90.1
	boolTrue, boolFalse                         = true, false
	timeRFC3339                                 = "2017-05-30T18:53:27.154Z"
	decoderBase                                 = map[string]interface{}{
		"true":    boolTrue,
		"str":     str,
		"fl32":    float32(5.4),
		"fl64":    fl64,
		"intfl32": float32(intFl32),
		"intfl64": float64(intFl64),
		"int":     integer,
		"strArr":  []interface{}{"c", "d"},
		"time":    timeRFC3339,
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"false":  boolFalse,
				"str":    str2,
				"fl32":   float32(78.4),
				"fl64":   fl64Also,
				"int":    integer2,
				"strArr": []interface{}{"k", "d"},
				"intArr": []interface{}{1, 2},
			},
		},
	}
)

type testStr struct {
	key  string
	keys []string
	out  interface{}
	err  error
}

func TestFloat64(t *testing.T) {
	for _, test := range []testStr{
		{key: "fl64", keys: []string{"a", "b"}, out: fl64Also, err: nil},
		{key: "fl64", keys: []string{}, out: fl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: 0.0, err: errFetch("missing", []string{"a", "b"})},
		{key: "str", keys: []string{"a", "b"}, out: 0.0, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.Float64(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestFloat64Ptr(t *testing.T) {
	var outnil *float64
	for _, test := range []testStr{
		{key: "fl64", keys: []string{"a", "b"}, out: &fl64Also, err: nil},
		{key: "fl64", keys: []string{}, out: &fl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.Float64Ptr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestIntPtr(t *testing.T) {
	var outnil *int
	for _, test := range []testStr{
		{key: "intfl32", keys: []string{}, out: &intFl32, err: nil},
		{key: "intfl64", keys: []string{}, out: &intFl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.IntPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInt64Ptr(t *testing.T) {
	var outnil *int64
	int64Fl64 := int64(intFl64)
	for _, test := range []testStr{
		{key: "intfl64", keys: []string{}, out: &int64Fl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.Int64Ptr(decoderBase, test.key, test.keys...)
		assert.Equal(t, test.out, out)
		assert.Equal(t, test.err, decoder.Err)
	}
}

func TestInt(t *testing.T) {
	for _, test := range []testStr{
		{key: "intfl32", keys: []string{}, out: intFl32, err: nil},
		{key: "intfl64", keys: []string{}, out: intFl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: 0, err: errFetch("missing", []string{"a", "b"})},
		{key: "str", keys: []string{"a", "b"}, out: 0, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.Int(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStrPtr(t *testing.T) {
	var outnil *string
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: &str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: &str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: errFetch("int", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.StringPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStr(t *testing.T) {
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: "", err: errFetch("missing", []string{"a", "b"})},
		{key: "int", keys: []string{"a", "b"}, out: "", err: errFetch("int", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.String(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStrArray(t *testing.T) {
	var outnil []string
	for _, test := range []testStr{
		{key: "strArr", keys: []string{}, out: []string{"c", "d"}, err: nil},
		{key: "strArr", keys: []string{"a", "b"}, out: []string{"k", "d"}, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: errFetch("str", []string{"a", "b"})},
		{key: "intArr", keys: []string{"a", "b"}, out: outnil, err: errFetch("intArr", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.StringArr(decoderBase, test.key, test.keys...)
		assert.Equal(t, test.err, decoder.Err)
		assert.Equal(t, test.out, out)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInterface(t *testing.T) {
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: nil, err: nil},
	} {
		decoder := ManualDecoder{}
		out := decoder.Interface(decoderBase, test.key, test.keys...)
		assert.Equal(t, test.out, out)
		assert.Equal(t, test.err, decoder.Err)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInterfaceArray(t *testing.T) {
	var outnil []interface{}
	for _, test := range []testStr{
		{key: "strArr", keys: []string{"a", "b"}, out: []interface{}{"k", "d"}, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: errFetch("int", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.InterfaceArr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}
func TestBoolPtr(t *testing.T) {
	var outnil *bool
	for _, test := range []testStr{
		{key: "true", keys: []string{}, out: &boolTrue, err: nil},
		{key: "false", keys: []string{"a", "b"}, out: &boolFalse, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: errFetch("int", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.BoolPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}
func TestMapStr(t *testing.T) {
	var outnil map[string]interface{}
	for _, test := range []testStr{
		{key: "a", keys: []string{}, out: decoderBase["a"], err: nil},
		{key: "b", keys: []string{"a"}, out: decoderBase["a"].(map[string]interface{})["b"], err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: errFetch("str", []string{"a", "b"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.MapStr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestTimeRFC3339(t *testing.T) {
	var outZero time.Time
	tp := time.Date(2017, 5, 30, 18, 53, 27, 154*1e6, time.UTC)
	for _, test := range []testStr{
		{key: "time", keys: []string{}, out: tp, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outZero, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outZero, err: errFetch("str", []string{"a", "b"})},
		{key: "b", keys: []string{"a"}, out: outZero, err: errFetch("b", []string{"a"})},
	} {
		decoder := ManualDecoder{}
		out := decoder.TimeRFC3339(decoderBase, test.key, test.keys...)
		assert.InDelta(t, out.Unix(), test.out.(time.Time).Unix(), time.Millisecond.Seconds()*10)
		assert.Equal(t, decoder.Err, test.err)
	}
}
