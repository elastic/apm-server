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
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

type testcase struct {
	name     string
	errorKey string
	data     string
}

func reader(t *testing.T, typ string) io.Reader {
	p := filepath.Join("..", "..", "..", "testdata", "intake-v2", fmt.Sprintf("%s.ndjson", typ))
	r, err := os.Open(p)
	require.NoError(t, err)
	return r
}

func TestMetadataSetResetIsSet(t *testing.T) {
	var m metadataRoot
	modeldecodertest.DecodeData(t, reader(t, "metadata"), "metadata", &m)
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
	assert.Equal(t, user{}, m.Metadata.User)
	assert.Empty(t, m.Metadata.Labels)
	assert.Empty(t, m.Metadata.Process.Pid)
	assert.Empty(t, m.Metadata.Process.Ppid)
	assert.Empty(t, m.Metadata.Process.Title)
	// test that array len is set to zero, but not capacity
	assert.Empty(t, m.Metadata.Process.Argv)
	assert.Greater(t, cap(m.Metadata.Process.Argv), 0)
}

func TestResetMetadataOnRelease(t *testing.T) {
	inp := `{"metadata":{"service":{"name":"service-a"}}}`
	m := fetchMetadataRoot()
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(m))
	require.True(t, m.IsSet())
	releaseMetadataRoot(m)
	assert.False(t, m.IsSet())
}

func TestDecodeMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		decodeFn func(decoder.Decoder, *model.Metadata) error
	}{
		{name: "decodeMetadata", decodeFn: DecodeMetadata,
			input: `{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`},
		{name: "decodeNestedMetadata", decodeFn: DecodeNestedMetadata,
			input: `{"metadata":{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}}`},
	} {
		t.Run("decode", func(t *testing.T) {
			var out model.Metadata
			dec := decoder.NewJSONIteratorDecoder(strings.NewReader(tc.input))
			require.NoError(t, tc.decodeFn(dec, &out))
			assert.Equal(t, model.Metadata{Service: model.Service{
				Name:  "user-service",
				Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

			err := tc.decodeFn(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decode")
		})

		t.Run("validate", func(t *testing.T) {
			inp := `{}`
			var out model.Metadata
			err := tc.decodeFn(decoder.NewJSONIteratorDecoder(strings.NewReader(inp)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation")
		})
	}
}

func TestDecodeMapToMetadataModel(t *testing.T) {
	// setup:
	// create initialized modeldecoder and empty model metadata
	// map modeldecoder to model metadata and manually set
	// enhanced data that are never set by the modeldecoder
	var m metadata
	modeldecodertest.SetStructValues(&m, "init", 5000, false, time.Now())
	var modelM model.Metadata
	ip := net.ParseIP("127.0.0.1")
	modelM.System.IP, modelM.Client.IP = ip, ip
	mapToMetadataModel(&m, &modelM)

	exceptions := func(key string) bool {
		return strings.HasPrefix(key, "UserAgent")
	}

	// iterate through model and assert values are set
	modeldecodertest.AssertStructValues(t, &modelM, exceptions, "init", 5000, false, ip, time.Now())

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(&m, "overwritten", 12, true, time.Now())
	mapToMetadataModel(&m, &modelM)
	modeldecodertest.AssertStructValues(t, &modelM, exceptions, "overwritten", 12, true, ip, time.Now())

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(&m)
	mapToMetadataModel(&m, &modelM)
	modeldecodertest.AssertStructValues(t, &modelM, exceptions, "overwritten", 12, true, ip, time.Now())
}

func TestMetadataValidationRules(t *testing.T) {
	testMetadata := func(t *testing.T, key string, tc testcase) {
		var m metadata
		r := reader(t, "metadata")
		modeldecodertest.DecodeDataWithReplacement(t, r, "metadata", key, tc.data, &m)

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
			{name: "id-string-max-len-exceeded", errorKey: "max", data: `{"id":"` + modeldecodertest.BuildString(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata(t, "user", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "name-valid-lower", data: `"name":"abcdefghijklmnopqrstuvwxyz"`},
			{name: "name-valid-upper", data: `"name":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`},
			{name: "name-valid-digits", data: `"name":"0123456789"`},
			{name: "name-valid-special", data: `"name":"_ -"`},
			{name: "name-asterisk", errorKey: "pattern(regexpAlphaNumericExt)", data: `"name":"abc*"`},
			{name: "name-dot", errorKey: "pattern(regexpAlphaNumericExt)", data: `"name":"abc."`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"agent":{"name":"go","version":"1.0"},` + tc.data + `}`
				testMetadata(t, "service", tc)
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
				testMetadata(t, "labels", tc)
			})
		}
	})

	t.Run("max-len", func(t *testing.T) {
		// check that `max` on strings is respected on an arbitrary field
		for _, tc := range []testcase{
			{name: "title-max-len", data: `{"pid":1,"title":"` + modeldecodertest.BuildString(1024) + `"}`},
			{name: "title-max-len-exceeded", errorKey: "max",
				data: `{"pid":1,"title":"` + modeldecodertest.BuildString(1025) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testMetadata(t, "process", tc)
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
	})
}
