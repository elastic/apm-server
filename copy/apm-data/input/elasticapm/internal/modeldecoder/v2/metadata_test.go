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

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-data/model/modelpb"
)

func isMetadataException(key string) bool {
	return isUnmappedMetadataField(key) || isEventField(key) || isIgnoredPrefix(key)
}

func isIgnoredPrefix(key string) bool {
	ignore := []string{"labels", "numeric_labels", "GlobalLabels", "GlobalNumericLabels"}
	for _, k := range ignore {
		if strings.HasPrefix(key, k) {
			return true
		}
	}
	return false
}

// unmappedMetadataFields holds the list of model fields that have no equivalent
// in the metadata input type.
func isUnmappedMetadataField(key string) bool {
	switch key {
	case
		"child_ids",
		"client.domain",
		"client",
		"client.ip",
		"client.port",
		"cloud.origin",
		"code",
		"code.stacktrace",
		"container.runtime",
		"container.image_name",
		"container.image_tag",
		"container.name",
		"data_stream",
		"data_stream.type",
		"data_stream.dataset",
		"data_stream.namespace",
		"destination",
		"destination.address",
		"destination.ip",
		"destination.port",
		"ECSVersion",
		"faas",
		"faas.id",
		"faas.cold_start",
		"faas.execution",
		"faas.trigger_type",
		"faas.trigger_request_id",
		"faas.name",
		"faas.version",
		"http",
		"http.request",
		"http.response",
		"http.version",
		"message",
		"network",
		"network.connection",
		"network.connection.subtype",
		"network.carrier",
		"network.carrier.name",
		"network.carrier.mcc",
		"network.carrier.mnc",
		"network.carrier.icc",
		"observer",
		"observer.ephemeral_id",
		"observer.hostname",
		"observer.id",
		"observer.name",
		"observer.type",
		"observer.version",
		"observer.version_major",
		"parent_id",
		"process.command_line",
		"process.executable",
		"process.thread",
		"process.thread.id",
		"process.thread.name",
		"processor",
		"processor.event",
		"processor.name",
		"device",
		"device.id",
		"device.model",
		"device.model.name",
		"device.model.identifier",
		"device.manufacturer",
		"host.os.full",
		"host.os.type",
		"host.os.name",
		"host.os.version",
		"host.id",
		"host.ip",
		"host.type",
		"user_agent",
		"user_agent.name",
		"user_agent.original",
		"event",
		"event.duration",
		"event.outcome",
		"event.success_count",
		"event.success_count.sum",
		"event.success_count.count",
		"event.dataset",
		"event.severity",
		"event.action",
		"event.kind",
		"event.type",
		"event.category",
		"log",
		"log.level",
		"log.logger",
		"log.origin",
		"log.origin.functionName",
		"log.origin.file",
		"log.origin.file.name",
		"log.origin.file.line",
		"service.origin",
		"service.origin.id",
		"service.origin.name",
		"service.origin.version",
		"service.target",
		"service.target.name",
		"service.target.type",
		"session.id",
		"session",
		"session.sequence",
		"source",
		"source.domain",
		"source.ip",
		"source.port",
		"source.nat",
		"timestamp",
		"trace",
		"trace.id",
		"url",
		"url.original",
		"url.scheme",
		"url.full",
		"url.domain",
		"url.port",
		"url.path",
		"url.query",
		"url.fragment",
		"system",
		"system.process",
		"system.process.state",
		"system.process.cmdline",
		"system.process.cpu.start_time",
		"system.filesystem",
		"system.filesystem.mount_point":
		return true
	}
	return false
}

func isEventField(key string) bool {
	for _, prefix := range []string{"error", "metricset", "span", "transaction"} {
		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return true
		}
	}
	return false
}

func initializedInputMetadata(values *modeldecodertest.Values) (metadata, *modelpb.APMEvent) {
	var input metadata
	var out modelpb.APMEvent
	modeldecodertest.SetStructValues(&input, values)
	mapToMetadataModel(&input, &out)
	modeldecodertest.SetStructValues(&out, values, func(key string, field, value reflect.Value) bool {
		return isUnmappedMetadataField(key) || isEventField(key)
	})
	out.Client = &modelpb.Client{Ip: values.IP}
	return input, &out
}

func TestDecodeMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		decodeFn func(decoder.Decoder, *modelpb.APMEvent) error
	}{
		{name: "decodeMetadata", decodeFn: DecodeMetadata,
			input: `{"labels":{"a":"b","c":true,"d":1234,"e":1234.11},"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`},
		{name: "decodeNestedMetadata", decodeFn: DecodeNestedMetadata,
			input: `{"metadata":{"labels":{"a":"b","c":true,"d":1234,"e":1234.11},"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}}`},
	} {
		t.Run("decode", func(t *testing.T) {
			var out modelpb.APMEvent
			dec := decoder.NewJSONDecoder(strings.NewReader(tc.input))
			require.NoError(t, tc.decodeFn(dec, &out))
			assert.Equal(t, &modelpb.APMEvent{
				Service: &modelpb.Service{Name: "user-service"},
				Agent:   &modelpb.Agent{Name: "go", Version: "1.0.0"},
				Labels: modelpb.Labels{
					"a": {Global: true, Value: "b"},
					"c": {Global: true, Value: "true"},
				},
				NumericLabels: modelpb.NumericLabels{
					"d": {Global: true, Value: float64(1234)},
					"e": {Global: true, Value: float64(1234.11)},
				},
			}, &out)

			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decode")
		})

		t.Run("validate", func(t *testing.T) {
			inp := `{}`
			var out modelpb.APMEvent
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
		mapToMetadataModel(&input, out)
		modeldecodertest.AssertStructValues(t, &out, isMetadataException, otherVal)

		// map an empty modeldecoder metadata to the model
		// and assert values are unchanged
		input.Reset()
		modeldecodertest.SetZeroStructValues(&input)
		mapToMetadataModel(&input, out)
		modeldecodertest.AssertStructValues(t, &out, isMetadataException, otherVal)
	})

	t.Run("reused-memory", func(t *testing.T) {
		var out2 modelpb.APMEvent
		defaultVal := modeldecodertest.DefaultValues()
		input, out1 := initializedInputMetadata(defaultVal)

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
		out2.Host.Ip = []*modelpb.IP{defaultVal.IP}
		if out2.Client == nil {
			out2.Client = &modelpb.Client{}
		}
		out2.Client.Ip = defaultVal.IP
		if out2.Source == nil {
			out2.Source = &modelpb.Source{}
		}
		out2.Source.Ip = defaultVal.IP
		modeldecodertest.AssertStructValues(t, &out2, isMetadataException, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, isMetadataException, defaultVal)
	})

	t.Run("system", func(t *testing.T) {
		var input metadata
		var out modelpb.APMEvent
		// full input information
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.System.ConfiguredHostname.Set("configured-host")
		input.System.DetectedHostname.Set("detected-host")
		input.System.DeprecatedHostname.Set("deprecated-host")
		input.System.HostID.Set("host-id")
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.Host.Name)
		assert.Equal(t, "detected-host", out.Host.Hostname)
		assert.Equal(t, "host-id", out.Host.Id)
		// no detected-host information
		out = modelpb.APMEvent{}
		input.System.DetectedHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Equal(t, "configured-host", out.Host.Name)
		assert.Empty(t, out.Host.Hostname)
		// no configured-host information
		out = modelpb.APMEvent{}
		input.System.ConfiguredHostname.Reset()
		mapToMetadataModel(&input, &out)
		assert.Empty(t, out.Host.Name)
		assert.Equal(t, "deprecated-host", out.Host.Hostname)
		// no host information given
		out = modelpb.APMEvent{}
		input.System.DeprecatedHostname.Reset()
		assert.Empty(t, out.GetHost().GetName())
		assert.Empty(t, out.GetHost().GetHostname())
	})
}
