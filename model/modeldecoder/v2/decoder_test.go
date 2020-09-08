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
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestResetModelOnRelease(t *testing.T) {
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

func TestMappingToModel(t *testing.T) {
	// setup:
	// create initialized modeldecoder and empty model metadata
	// map modeldecoder to model metadata and manually set
	// enhanced data that are never set by the modeldecoder
	val := reflect.New(reflect.TypeOf(metadata{}))
	modeldecodertest.SetStructValues(val, "init", 5000)
	m := val.Interface().(*metadata)
	var modelM model.Metadata
	modelM.System.IP, modelM.Client.IP = net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")
	modelM.UserAgent.Original, modelM.UserAgent.Name = "Firefox/15.0.1", "Firefox/15.0.1"
	mapToMetadataModel(m, &modelM)

	// iterate through model and assert values are set
	assertStructValues(t, reflect.ValueOf(&modelM), "init", 5000)

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(val, "overwritten", 12)
	m = val.Interface().(*metadata)
	mapToMetadataModel(m, &modelM)
	assertStructValues(t, reflect.ValueOf(&modelM), "overwritten", 12)

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(val)
	m = val.Interface().(*metadata)
	mapToMetadataModel(m, &modelM)
	assertStructValues(t, reflect.ValueOf(&modelM), "overwritten", 12)
}

func assertStructValues(t *testing.T, val reflect.Value, s string, i int) {
	modeldecodertest.IterateStruct(val, func(f reflect.Value, key string) {
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{s: s}
		case common.MapStr:
			newVal = common.MapStr{s: s}
		case []string:
			newVal = []string{s}
		case []int:
			newVal = []int{i, i}
		case string:
			newVal = s
		case int:
			newVal = i
		case *int:
			iptr := f.Interface().(*int)
			fVal = *iptr
			newVal = i
		case net.IP:
		default:
			if f.Type().Kind() == reflect.Struct {
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		if strings.HasPrefix(key, "UserAgent") || key == "Client.IP" || key == "System.IP" {
			// these values are not set by modeldecoder
			return
		}
		assert.Equal(t, newVal, fVal, key)
	})
}
