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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

// unmappedMetadataFields holds the list of model fields that have no equivalent
// in the metadata input type.
func isUnmappedMetadataField(key string) bool {
	switch key {
	case
		"Client.Domain",
		"Client.IP",
		"Client.Port",
		"Container.Runtime",
		"Container.ImageName",
		"Container.ImageTag",
		"Container.Name",
		"Network",
		"Network.ConnectionType",
		"Network.Carrier",
		"Network.Carrier.Name",
		"Network.Carrier.MCC",
		"Network.Carrier.MNC",
		"Network.Carrier.ICC",
		"Process.CommandLine",
		"Process.Executable",
		"System.FullPlatform",
		"System.ID",
		"System.IP",
		"System.OSType",
		"System.Type",
		"UserAgent",
		"UserAgent.Name",
		"UserAgent.Original":
		return true
	}
	return false
}

func initializedMetadata() *model.Metadata {
	_, metadata := initializedInputMetadata(modeldecodertest.DefaultValues())
	return &metadata
}

func initializedInputMetadata(values *modeldecodertest.Values) (metadata, model.Metadata) {
	var input metadata
	var out model.Metadata
	modeldecodertest.SetStructValues(&input, values)
	mapToMetadataModel(&input, &out)
	modeldecodertest.SetStructValues(&out, values, func(key string, field, value reflect.Value) bool {
		return isUnmappedMetadataField(key)
	})
	return input, out
}

func TestResetMetadataOnRelease(t *testing.T) {
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
			assert.Equal(t, model.Metadata{
				Service: model.Service{Name: "user-service"},
				Agent:   model.Agent{Name: "go", Version: "1.0.0"},
			}, out)

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

func TestDecodeMapToMetadataModel(t *testing.T) {

	t.Run("overwrite", func(t *testing.T) {
		// setup:
		// create initialized modeldecoder and empty model metadata
		// map modeldecoder to model metadata and manually set
		// enhanced data that are never set by the modeldecoder
		defaultVal := modeldecodertest.DefaultValues()
		input, out := initializedInputMetadata(defaultVal)

		exceptions := func(key string) bool {
			return isUnmappedMetadataField(key)
		}

		// iterate through model and assert values are set
		modeldecodertest.AssertStructValues(t, &out, exceptions, defaultVal)

		// overwrite model metadata with specified Values
		// then iterate through model and assert values are overwritten
		otherVal := modeldecodertest.NonDefaultValues()
		// System.IP and Client.IP are not set by decoder,
		// therefore their values are not updated
		otherVal.Update(defaultVal.IP)
		input.Reset()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetadataModel(&input, &out)
		modeldecodertest.AssertStructValues(t, &out, exceptions, otherVal)

		// map an empty modeldecoder metadata to the model
		// and assert values are unchanged
		input.Reset()
		modeldecodertest.SetZeroStructValues(&input)
		mapToMetadataModel(&input, &out)
		modeldecodertest.AssertStructValues(t, &out, exceptions, otherVal)
	})

	t.Run("reused-memory", func(t *testing.T) {
		var out2 model.Metadata
		defaultVal := modeldecodertest.DefaultValues()
		input, out1 := initializedInputMetadata(defaultVal)

		exceptions := func(key string) bool {
			return isUnmappedMetadataField(key)
		}
		// iterate through model and assert values are set
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// overwrite model metadata with specified Values
		// then iterate through model and assert values are overwritten
		otherVal := modeldecodertest.NonDefaultValues()
		// System.IP and Client.IP are not set by decoder,
		// therefore their values are not updated
		otherVal.Update(defaultVal.IP)
		input.Reset()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetadataModel(&input, &out2)
		out2.System.IP, out2.Client.IP = defaultVal.IP, defaultVal.IP
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("system", func(t *testing.T) {
		var input metadata
		var out model.Metadata
		// full input information
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.System.ConfiguredHostname.Set("configured-host")
		input.System.DetectedHostname.Set("detected-host")
		input.System.DeprecatedHostname.Set("deprecated-host")
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.System.ConfiguredHostname)
		assert.Equal(t, "detected-host", out.System.DetectedHostname)
		// no detected-host information
		out = model.Metadata{}
		input.System.DetectedHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.System.ConfiguredHostname)
		assert.Empty(t, out.System.DetectedHostname)
		// no configured-host information
		out = model.Metadata{}
		input.System.ConfiguredHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Empty(t, out.System.ConfiguredHostname)
		assert.Equal(t, "deprecated-host", out.System.DetectedHostname)
		// no host information given
		out = model.Metadata{}
		input.System.DeprecatedHostname.Reset()
		assert.Empty(t, out.System.ConfiguredHostname)
		assert.Empty(t, out.System.DetectedHostname)

	})
}
