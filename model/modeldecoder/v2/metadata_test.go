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
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

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
