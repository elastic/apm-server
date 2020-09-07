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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/beats/v7/libbeat/common"
)

var testMinValidMetadata = `
{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`

func TestIsSet(t *testing.T) {
	inp := `{"cloud":{"availability_zone":"eu-west-3","instance":{"id":"1234"}}}`
	var m metadata
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(&m))
	assert.True(t, m.IsSet())
	assert.True(t, m.Cloud.IsSet())
	assert.True(t, m.Cloud.AvailabilityZone.IsSet())
	assert.True(t, m.Cloud.Instance.ID.IsSet())
	assert.False(t, m.Cloud.Instance.Name.IsSet())
}

func TestSetReset(t *testing.T) {
	var m metadataRoot
	r, err := os.Open("../../../testdata/intake-v2/metadata.ndjson")
	require.NoError(t, err)
	require.NoError(t, decoder.NewJSONIteratorDecoder(r).Decode(&m))
	require.True(t, m.IsSet())
	require.True(t, m.Metadata.Cloud.IsSet())
	require.NotEmpty(t, m.Metadata.Labels)
	require.True(t, m.Metadata.Process.IsSet())
	require.True(t, m.Metadata.Service.IsSet())
	require.True(t, m.Metadata.System.IsSet())
	require.True(t, m.Metadata.User.IsSet())
	// call Reset and ensure initial state, except for array capacity
	m.Reset()
	assert.False(t, m.IsSet())
	assert.Equal(t, metadataCloud{}, m.Metadata.Cloud)
	assert.Equal(t, metadataService{}, m.Metadata.Service)
	assert.Equal(t, metadataSystem{}, m.Metadata.System)
	assert.Equal(t, metadataUser{}, m.Metadata.User)
	assert.Empty(t, m.Metadata.Labels)
	assert.Empty(t, m.Metadata.Process.Pid)
	assert.Empty(t, m.Metadata.Process.Ppid)
	assert.Empty(t, m.Metadata.Process.Title)
	// test that array len is set to zero, but not capacity
	assert.Empty(t, m.Metadata.Process.Argv)
	assert.Greater(t, cap(m.Metadata.Process.Argv), 0)
}

func TestValidationRules(t *testing.T) {
	type testcase struct {
		name     string
		errorKey string
		data     string
	}

	strBuilder := func(n int) string {
		b := make([]rune, n)
		for i := range b {
			b[i] = '⌘'
		}
		return string(b)
	}

	testMetadata := func(key string, tc testcase) {
		// load minimal required data
		// set testcase data for given key
		var data map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(testMinValidMetadata), &data))
		var keyData map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(tc.data), &keyData))
		data[key] = keyData
		// unmarshal data into metdata struct
		var m metadata
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.NoError(t, decoder.NewJSONIteratorDecoder(bytes.NewReader(b)).Decode(&m))
		// run validation and checks
		err = m.validate()
		if tc.errorKey == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorKey)
		}
	}

	t.Run("user", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "id-string", data: `{"id":"user123"}`},
			{name: "id-int", data: `{"id":44}`},
			{name: "id-float", errorKey: "types", data: `{"id":45.6}`},
			{name: "id-bool", errorKey: "types", data: `{"id":true}`},
			{name: "id-string-max-len", data: `{"id":"` + strBuilder(1024) + `"}`},
			{name: "id-string-max-len", errorKey: "max", data: `{"id":"` + strBuilder(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata("user", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "name-valid-lower", data: `"name":"abcdefghijklmnopqrstuvwxyz"`},
			{name: "name-valid-upper", data: `"name":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`},
			{name: "name-valid-digits", data: `"name":"0123456789"`},
			{name: "name-valid-special", data: `"name":"_ -"`},
			{name: "name-asterisk", errorKey: "service.name", data: `"name":"abc*"`},
			{name: "name-dot", errorKey: "service.name", data: `"name":"abc."`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"agent":{"name":"go","version":"1.0"},` + tc.data + `}`
				testMetadata("service", tc)
			})
		}
	})

	t.Run("labels", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "valid", data: `{"k1":"v1","k2":2.3,"k3":3,"k4":true,"k5":null}`},
			{name: "restricted-type", errorKey: "typesVals", data: `{"k1":{"k2":"v1"}}`},
			{name: "key-dot", errorKey: "patternKeys", data: `{"k.1":"v1"}`},
			{name: "key-asterisk", errorKey: "patternKeys", data: `{"k*1":"v1"}`},
			{name: "key-quotemark", errorKey: "patternKeys", data: `{"k\"1":"v1"}`},
			{name: "max-len", data: `{"k1":"` + strBuilder(1024) + `"}`},
			{name: "max-len-exceeded", errorKey: "maxVals", data: `{"k1":"` + strBuilder(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata("labels", tc)
			})
		}
	})

	t.Run("max-len", func(t *testing.T) {
		// check that `max` on strings is respected on an arbitrary field
		for _, tc := range []testcase{
			{name: "title-max-len", data: `{"pid":1,"title":"` + strBuilder(1024) + `"}`},
			{name: "title-max-len-exceeded", errorKey: "max",
				data: `{"pid":1,"title":"` + strBuilder(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata("process", tc)
			})
		}
	})

	t.Run("required", func(t *testing.T) {
		// setup: create full metadata struct with arbitrary values set
		typ := reflect.TypeOf(metadata{})
		val := reflect.New(typ)
		iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
			var newVal interface{}
			switch val := f.Interface().(type) {
			case map[string]interface{}:
				newVal = map[string]interface{}{"k1": "v1"}
			case common.MapStr:
				newVal = common.MapStr{"k1": "v1"}
			case []string:
				newVal = []string{"a", "b"}
			case []int:
				newVal = []int{1, 2, 3}
			case nullable.String:
				val.Set("teststring")
				newVal = val
			case nullable.Int:
				val.Set(123)
				newVal = val
			case nullable.Interface:
				val.Set("testinterface")
				newVal = val
			default:
				if f.Type().Kind() == reflect.Struct {
					return
				}
				panic(fmt.Sprintf("initStruct: unhandled type %T", f.Type().Kind()))
			}
			f.Set(reflect.ValueOf(newVal))
		})

		// test vanilla struct is valid
		metadata := val.Interface().(*metadata)
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
		iterateStruct(val.Elem(), "", func(f reflect.Value, key string) {
			original := reflect.ValueOf(f.Interface())
			defer f.Set(original) // reset original value
			f.Set(reflect.Zero(f.Type()))
			err := metadata.validate()
			if _, ok := requiredKeys[key]; ok {
				require.Error(t, err, key)
				assert.Contains(t, err.Error(), key)
			} else {
				assert.NoError(t, err, key)
			}
		})
	})
}

func iterateStruct(v reflect.Value, key string, cb func(f reflect.Value, fKey string)) {
	t := v.Type()
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("iterateStruct: invalid typ %T", t.Kind()))
	}
	if key != "" {
		key += "."
	}
	var fKey string
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		stf := t.Field(i)
		fTyp := stf.Type
		fKey = fmt.Sprintf("%s%s", key, jsonName(stf))

		if fTyp.Kind() == reflect.Struct {
			switch f.Interface().(type) {
			case nullable.String, nullable.Int, nullable.Interface:
			default:
				iterateStruct(f, fKey, cb)
			}
		}
		cb(f, fKey)
	}
}

func initStruct(f reflect.Value, fKey string) {
}

func jsonName(f reflect.StructField) string {
	tag, ok := f.Tag.Lookup("json")
	if !ok || tag == "-" {
		return ""
	}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
