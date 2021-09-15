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

func isMetadataException(key string) bool {
	return isUnmappedMetadataField(key) || isEventField(key)
}

// unmappedMetadataFields holds the list of model fields that have no equivalent
// in the metadata input type.
func isUnmappedMetadataField(key string) bool {
	switch key {
	case
		"Child",
		"Child.ID",
		"Client.Domain",
		"Client.IP",
		"Client.Port",
		"Cloud.Origin",
		"Container.Runtime",
		"Container.ImageName",
		"Container.ImageTag",
		"Container.Name",
		"DataStream",
		"DataStream.Type",
		"DataStream.Dataset",
		"DataStream.Namespace",
		"Destination",
		"Destination.Address",
		"Destination.IP",
		"Destination.Port",
		"ECSVersion",
		"FAAS",
		"FAAS.Coldstart",
		"FAAS.Execution",
		"FAAS.TriggerType",
		"FAAS.TriggerRequestID",
		"HTTP",
		"HTTP.Request",
		"HTTP.Response",
		"HTTP.Version",
		"Message",
		"Network",
		"Network.Connection",
		"Network.Connection.Subtype",
		"Network.Carrier",
		"Network.Carrier.Name",
		"Network.Carrier.MCC",
		"Network.Carrier.MNC",
		"Network.Carrier.ICC",
		"Observer",
		"Observer.EphemeralID",
		"Observer.Hostname",
		"Observer.ID",
		"Observer.Name",
		"Observer.Type",
		"Observer.Version",
		"Observer.VersionMajor",
		"Parent",
		"Parent.ID",
		"Process.CommandLine",
		"Process.Executable",
		"Processor",
		"Processor.Event",
		"Processor.Name",
		"Host.OS.Full",
		"Host.OS.Type",
		"Host.ID",
		"Host.IP",
		"Host.Type",
		"UserAgent",
		"UserAgent.Name",
		"UserAgent.Original",
		"Event",
		"Event.Duration",
		"Event.Outcome",
		"Service.Origin",
		"Service.Origin.ID",
		"Service.Origin.Name",
		"Service.Origin.Version",
		"Session.ID",
		"Session",
		"Session.Sequence",
		"Source.Domain",
		"Source.IP",
		"Source.Port",
		"Trace",
		"Trace.ID",
		"URL",
		"URL.Original",
		"URL.Scheme",
		"URL.Full",
		"URL.Domain",
		"URL.Port",
		"URL.Path",
		"URL.Query",
		"URL.Fragment":
		return true
	}
	return false
}

func isEventField(key string) bool {
	for _, prefix := range []string{"Error", "Metricset", "ProfileSample", "Span", "Transaction"} {
		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return true
		}
	}
	return false
}

func initializedInputMetadata(values *modeldecodertest.Values) (metadata, model.APMEvent) {
	var input metadata
	var out model.APMEvent
	modeldecodertest.SetStructValues(&input, values)
	mapToMetadataModel(&input, &out)
	modeldecodertest.SetStructValues(&out, values, func(key string, field, value reflect.Value) bool {
		return isUnmappedMetadataField(key) || isEventField(key)
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
		decodeFn func(decoder.Decoder, *model.APMEvent) error
	}{
		{name: "decodeMetadata", decodeFn: DecodeMetadata,
			input: `{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`},
		{name: "decodeNestedMetadata", decodeFn: DecodeNestedMetadata,
			input: `{"metadata":{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}}`},
	} {
		t.Run("decode", func(t *testing.T) {
			var out model.APMEvent
			dec := decoder.NewJSONDecoder(strings.NewReader(tc.input))
			require.NoError(t, tc.decodeFn(dec, &out))
			assert.Equal(t, model.APMEvent{
				Service: model.Service{Name: "user-service"},
				Agent:   model.Agent{Name: "go", Version: "1.0.0"},
			}, out)

			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decode")
		})

		t.Run("validate", func(t *testing.T) {
			inp := `{}`
			var out model.APMEvent
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
		out.Timestamp = defaultVal.Time

		// iterate through model and assert values are set
		modeldecodertest.AssertStructValues(t, &out, isMetadataException, defaultVal)

		// overwrite model metadata with specified Values
		// then iterate through model and assert values are overwritten
		otherVal := modeldecodertest.NonDefaultValues()
		// System.IP and Client.IP are not set by decoder,
		// therefore their values are not updated
		otherVal.Update(defaultVal.IP)
		input.Reset()
		modeldecodertest.SetStructValues(&input, otherVal)
		out.Timestamp = otherVal.Time
		mapToMetadataModel(&input, &out)
		modeldecodertest.AssertStructValues(t, &out, isMetadataException, otherVal)

		// map an empty modeldecoder metadata to the model
		// and assert values are unchanged
		input.Reset()
		modeldecodertest.SetZeroStructValues(&input)
		mapToMetadataModel(&input, &out)
		modeldecodertest.AssertStructValues(t, &out, isMetadataException, otherVal)
	})

	t.Run("reused-memory", func(t *testing.T) {
		var out2 model.APMEvent
		defaultVal := modeldecodertest.DefaultValues()
		input, out1 := initializedInputMetadata(defaultVal)
		out1.Timestamp = defaultVal.Time

		// iterate through model and assert values are set
		modeldecodertest.AssertStructValues(t, &out1, isMetadataException, defaultVal)

		// overwrite model metadata with specified Values
		// then iterate through model and assert values are overwritten
		otherVal := modeldecodertest.NonDefaultValues()
		// System.IP and Client.IP are not set by decoder,
		// therefore their values are not updated
		otherVal.Update(defaultVal.IP)
		input.Reset()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToMetadataModel(&input, &out2)
		out2.Timestamp = otherVal.Time
		out2.Host.IP = defaultVal.IP
		out2.Client.IP = defaultVal.IP
		out2.Source.IP = defaultVal.IP
		modeldecodertest.AssertStructValues(t, &out2, isMetadataException, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, isMetadataException, defaultVal)
	})

	t.Run("system", func(t *testing.T) {
		var input metadata
		var out model.APMEvent
		// full input information
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.System.ConfiguredHostname.Set("configured-host")
		input.System.DetectedHostname.Set("detected-host")
		input.System.DeprecatedHostname.Set("deprecated-host")
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.Host.Name)
		assert.Equal(t, "detected-host", out.Host.Hostname)
		// no detected-host information
		out = model.APMEvent{}
		input.System.DetectedHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.Host.Name)
		assert.Empty(t, out.Host.Hostname)
		// no configured-host information
		out = model.APMEvent{}
		input.System.ConfiguredHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Empty(t, out.Host.Name)
		assert.Equal(t, "deprecated-host", out.Host.Hostname)
		// no host information given
		out = model.APMEvent{}
		input.System.DeprecatedHostname.Reset()
		assert.Empty(t, out.Host.Name)
		assert.Empty(t, out.Host.Hostname)

	})
}
