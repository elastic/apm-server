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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func testdata(t *testing.T) io.Reader {
	r, err := os.Open("../../../testdata/intake-v3/metadata.ndjson")
	require.NoError(t, err)
	return r
}

func TestIsSet(t *testing.T) {
	data := `{"se":{"n":"user-service"}}`
	var m metadata
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(data)).Decode(&m))
	assert.True(t, m.IsSet())
	assert.True(t, m.Service.IsSet())
	assert.True(t, m.Service.Name.IsSet())
	assert.False(t, m.Service.Language.IsSet())
}

func TestSetReset(t *testing.T) {
	var m metadataRoot
	require.NoError(t, decoder.NewJSONIteratorDecoder(testdata(t)).Decode(&m))
	require.True(t, m.IsSet())
	require.NotEmpty(t, m.Metadata.Labels)
	require.True(t, m.Metadata.Service.IsSet())
	require.True(t, m.Metadata.User.IsSet())
	// call Reset and ensure initial state, except for array capacity
	m.Reset()
	assert.False(t, m.IsSet())
	assert.Equal(t, metadataService{}, m.Metadata.Service)
	assert.Equal(t, metadataUser{}, m.Metadata.User)
	assert.Empty(t, m.Metadata.Labels)
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

	testMetadata := func(t *testing.T, key string, tc testcase) {
		// load data
		// set testcase data for given key
		var data map[string]interface{}
		require.NoError(t, decoder.NewJSONIteratorDecoder(testdata(t)).Decode(&data))
		meta := data["m"].(map[string]interface{})
		var keyData map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(tc.data), &keyData))
		meta[key] = keyData

		// unmarshal data into metdata struct
		var m metadata
		b, err := json.Marshal(meta)
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
				testMetadata(t, "u", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "name-valid-lower", data: `"n":"abcdefghijklmnopqrstuvwxyz"`},
			{name: "name-valid-upper", data: `"n":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`},
			{name: "name-valid-digits", data: `"n":"0123456789"`},
			{name: "name-valid-special", data: `"n":"_ -"`},
			{name: "name-asterisk", errorKey: "se.n", data: `"n":"abc*"`},
			{name: "name-dot", errorKey: "se.n", data: `"n":"abc."`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"a":{"n":"go","ve":"1.0"},` + tc.data + `}`
				testMetadata(t, "se", tc)
			})
		}
	})

	t.Run("max-len", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "service-environment-max-len", data: `"en":"` + strBuilder(1024) + `"`},
			{name: "service-environment-max-len", errorKey: "max", data: `"en":"` + strBuilder(1025) + `"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"a":{"n":"go","ve":"1.0"},"n":"my-service",` + tc.data + `}`
				testMetadata(t, "se", tc)
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
				testMetadata(t, "l", tc)
			})
		}
	})

	t.Run("required", func(t *testing.T) {
		// setup: create full metadata struct with arbitrary values set
		val := reflect.New(reflect.TypeOf(metadata{}))
		modeldecodertest.InitStructValues(val)

		// test vanilla struct is valid
		metadata := val.Interface().(*metadata)
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
		modeldecodertest.SetZeroStructValue(val, func(key string) {
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
