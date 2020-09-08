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

//TODO(simitt): Implement transaction decoder tests

func TestResetModelOnRelease(t *testing.T) {
	inp := `{"metadata":{"service":{"name":"service-a"}}}`
	m := fetchMetadataRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(m))
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
			dec := decoder.NewJSONDecoder(strings.NewReader(tc.input))
			require.NoError(t, tc.decodeFn(dec, &out))
			assert.Equal(t, model.Metadata{Service: model.Service{
				Name:  "user-service",
				Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decode")
		})

		t.Run("validate", func(t *testing.T) {
			inp := `{}`
			var out model.Metadata
			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(inp)), &out)
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
	var m metadata
	modeldecodertest.SetStructValues(&m, "init", 5000)
	var modelM model.Metadata
	modelM.System.IP, modelM.Client.IP = net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")
	modelM.UserAgent.Original, modelM.UserAgent.Name = "Firefox/15.0.1", "Firefox/15.0.1"
	mapToMetadataModel(&m, &modelM)

	// iterate through model and assert values are set
	assertStructValues(t, &modelM, "init", 5000)

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(&m, "overwritten", 12)
	mapToMetadataModel(&m, &modelM)
	assertStructValues(t, &modelM, "overwritten", 12)

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(&m)
	mapToMetadataModel(&m, &modelM)
	assertStructValues(t, &modelM, "overwritten", 12)
}

func assertStructValues(t *testing.T, i interface{}, vStr string, vInt int) {
	modeldecodertest.IterateStruct(i, func(f reflect.Value, key string) {
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{vStr: vStr}
		case common.MapStr:
			newVal = common.MapStr{vStr: vStr}
		case []string:
			newVal = []string{vStr}
		case []int:
			newVal = []int{vInt, vInt}
		case string:
			newVal = vStr
		case int:
			newVal = vInt
		case *int:
			iptr := f.Interface().(*int)
			fVal = *iptr
			newVal = vInt
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
