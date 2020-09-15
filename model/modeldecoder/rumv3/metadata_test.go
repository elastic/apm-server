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

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/beats/v7/libbeat/common"
)

type testcase struct {
	name     string
	errorKey string
	data     string
}

func reader(t *testing.T, typ string) io.Reader {
	p := filepath.Join("..", "..", "..", "testdata", "intake-v3", fmt.Sprintf("%s.ndjson", typ))
	r, err := os.Open(p)
	require.NoError(t, err)
	return r
}

func TestSetResetIsSet(t *testing.T) {
	var m metadataRoot
	require.NoError(t, decoder.NewJSONIteratorDecoder(reader(t, "metadata")).Decode(&m))
	require.True(t, m.IsSet())
	require.NotEmpty(t, m.Metadata.Labels)
	require.True(t, m.Metadata.Service.IsSet())
	require.True(t, m.Metadata.User.IsSet())
	// call Reset and ensure initial state, except for array capacity
	m.Reset()
	assert.False(t, m.IsSet())
	assert.Equal(t, metadataService{}, m.Metadata.Service)
	assert.Equal(t, user{}, m.Metadata.User)
	assert.Empty(t, m.Metadata.Labels)
}

func TestMetadataResetModelOnRelease(t *testing.T) {
	inp := `{"m":{"se":{"n":"service-a"}}}`
	m := fetchMetadataRoot()
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(m))
	require.True(t, m.IsSet())
	releaseMetadataRoot(m)
	assert.False(t, m.IsSet())
}

func TestDecodeNestedMetadata(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		var out model.Metadata
		testMinValidMetadata := `{"m":{"se":{"n":"name","a":{"n":"go","ve":"1.0.0"}}}}`
		dec := decoder.NewJSONIteratorDecoder(strings.NewReader(testMinValidMetadata))
		require.NoError(t, DecodeNestedMetadata(dec, &out))
		assert.Equal(t, model.Metadata{Service: model.Service{
			Name:  "name",
			Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

		err := DecodeNestedMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		inp := `{}`
		var out model.Metadata
		err := DecodeNestedMetadata(decoder.NewJSONIteratorDecoder(strings.NewReader(inp)), &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})

}

func TestDecodeMetadataMappingToModel(t *testing.T) {
	expected := func(s string) model.Metadata {
		return model.Metadata{
			Service: model.Service{Name: s, Version: s, Environment: s,
				Agent:     model.Agent{Name: s, Version: s},
				Language:  model.Language{Name: s, Version: s},
				Runtime:   model.Runtime{Name: s, Version: s},
				Framework: model.Framework{Name: s, Version: s}},
			User:   model.User{Name: s, Email: s, ID: s},
			Labels: common.MapStr{s: s},
		}
	}

	// setup:
	// create initialized modeldecoder and empty model metadata
	// map modeldecoder to model metadata and manually set
	// enhanced data that are never set by the modeldecoder
	var m metadata
	modeldecodertest.SetStructValues(&m, "init", 5000, false)
	var modelM model.Metadata
	mapToMetadataModel(&m, &modelM)
	// iterate through model and assert values are set
	assert.Equal(t, expected("init"), modelM)

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(&m, "overwritten", 12, false)
	mapToMetadataModel(&m, &modelM)
	assert.Equal(t, expected("overwritten"), modelM)

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(&m)
	mapToMetadataModel(&m, &modelM)
	assert.Equal(t, expected("overwritten"), modelM)

}

func TestValidationRules(t *testing.T) {
	testMetadata := func(t *testing.T, key string, tc testcase) {
		var m metadata
		r := reader(t, "metadata")
		modeldecodertest.ReplaceData(t, r, "m", key, tc.data, &m)

		// run validation and checks
		err := m.validate()
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
			{name: "id-string-max-len", data: `{"id":"` + modeldecodertest.BuildString(1024) + `"}`},
			{name: "id-string-max-len", errorKey: "max", data: `{"id":"` + modeldecodertest.BuildString(1025) + `"}`},
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
			{name: "service-environment-max-len", data: `"en":"` + modeldecodertest.BuildString(1024) + `"`},
			{name: "service-environment-max-len", errorKey: "max", data: `"en":"` + modeldecodertest.BuildString(1025) + `"`},
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
			{name: "max-len", data: `{"k1":"` + modeldecodertest.BuildString(1024) + `"}`},
			{name: "max-len-exceeded", errorKey: "maxVals", data: `{"k1":"` + modeldecodertest.BuildString(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata(t, "l", tc)
			})
		}
	})

	t.Run("required", func(t *testing.T) {
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
				assert.Contains(t, err.Error(), key)
			} else {
				assert.NoError(t, err, key)
			}
		})
	})
}
