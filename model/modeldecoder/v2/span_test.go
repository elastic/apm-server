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
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestResetSpanOnRelease(t *testing.T) {
	inp := `{"span":{"name":"tr-a"}}`
	root := fetchSpanRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseSpanRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedSpan(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: time.Now(), Config: modeldecoder.Config{}}
		str := `{"span":{"duration":100,"id":"a-b-c","name":"s","parent_id":"parent-123","trace_id":"trace-ab","type":"db","start":143}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Span
		require.NoError(t, DecodeNestedSpan(dec, &input, &out))

		err := DecodeNestedSpan(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out model.Span
		err := DecodeNestedSpan(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToSpanModel(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")

	initializedMeta := func() *model.Metadata {
		var inputMeta metadata
		var meta model.Metadata
		modeldecodertest.SetStructValues(&inputMeta, "meta", 1, false, time.Now())
		mapToMetadataModel(&inputMeta, &meta)
		// initialize values that are not set by input
		meta.UserAgent = model.UserAgent{Name: "meta", Original: "meta"}
		meta.Client.IP = ip
		meta.System.IP = ip
		return &meta
	}

	t.Run("metadata", func(t *testing.T) {
		// do not overwrite metadata with zero span values
		exceptions := func(key string) bool { return false }
		var input span
		var out model.Span
		modeldecodertest.SetStructValues(&input, "overwritten", 5000, true, time.Now().Add(time.Hour))
		mapToSpanModel(&input, initializedMeta(), time.Now(), modeldecoder.Config{}, &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, "meta", 1, false, ip, time.Now())
	})

	t.Run("experimental", func(t *testing.T) {
		// experimental enabled
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: time.Now(), Config: modeldecoder.Config{Experimental: true}}
		str := `{"span":{"context":{"experimental":"exp"},"duration":100,"id":"a-b-c","name":"s","parent_id":"parent-123","trace_id":"trace-ab","type":"db","start":143}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Span
		require.NoError(t, DecodeNestedSpan(dec, &input, &out))
		assert.Equal(t, "exp", out.Experimental)

		// experimental disabled
		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: time.Now(), Config: modeldecoder.Config{Experimental: false}}
		str = `{"span":{"context":{"experimental":"exp"},"duration":100,"id":"a-b-c","name":"s","parent_id":"parent-123","trace_id":"trace-ab","type":"db","start":143}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		out = model.Span{}
		require.NoError(t, DecodeNestedSpan(dec, &input, &out))
		// experimental should only be set if allowed by configuration
		assert.Nil(t, out.Experimental)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// metadata are tested in the 'metadata' test
			// experimental is tested in test 'experimental'
			// RUM is tested in test 'span-values'
			// RepresentativeCount is tested further down in test 'sample-rate'
			for _, s := range []string{"Experimental", "RUM", "RepresentativeCount"} {
				if key == s {
					return true
				}
			}
			for _, s := range []string{"Metadata",
				"Stacktrace.Original", "Stacktrace.Sourcemap", "Stacktrace.ExcludeFromGrouping"} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}
		var input span
		var out model.Span
		eventTime, reqTime := time.Now(), time.Now().Add(time.Second)
		modeldecodertest.SetStructValues(&input, "overwritten", 5000, true, eventTime)
		mapToSpanModel(&input, initializedMeta(), reqTime, modeldecoder.Config{Experimental: true}, &out)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out, exceptions, "overwritten", 5000, true, ip, eventTime)
		assert.False(t, out.RUM)
	})

	t.Run("timestamp", func(t *testing.T) {
		var input span
		var out model.Span
		i := 5000
		reqTime := time.Now().Add(time.Hour)
		// add start to requestTime if eventTime is zero and start is given
		modeldecodertest.SetStructValues(&input, "init", i, true, time.Time{})
		mapToSpanModel(&input, initializedMeta(), reqTime, modeldecoder.Config{}, &out)
		timestamp := reqTime.Add(time.Duration((float64(i) + 0.5) * float64(time.Millisecond)))
		assert.Equal(t, timestamp, out.Timestamp)
		// set requestTime if eventTime is zero and start is not set
		out = model.Span{}
		modeldecodertest.SetStructValues(&input, "init", i, true, time.Time{})
		input.Start.Reset()
		mapToSpanModel(&input, initializedMeta(), reqTime, modeldecoder.Config{}, &out)
		require.Nil(t, out.Start)
		assert.Equal(t, reqTime, out.Timestamp)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input span
		var out model.Span
		modeldecodertest.SetStructValues(&input, "init", 5000, true, time.Now())
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToSpanModel(&input, initializedMeta(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 4.0, out.RepresentativeCount)
		// sample rate is not set
		out.RepresentativeCount = 0.0
		input.SampleRate.Reset()
		mapToSpanModel(&input, initializedMeta(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 0.0, out.RepresentativeCount)
		// sample rate is set to 0
		input.SampleRate.Set(0)
		mapToSpanModel(&input, initializedMeta(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 0.0, out.RepresentativeCount)
	})

	t.Run("type-subtype-action", func(t *testing.T) {
		for _, tc := range []struct {
			name                                 string
			inputType, inputSubtype, inputAction string
			typ, subtype, action                 string
		}{
			{name: "only-type", inputType: "xyz",
				typ: "xyz"},
			{name: "derive-subtype", inputType: "x.y",
				typ: "x", subtype: "y"},
			{name: "derive-subtype-action", inputType: "x.y.z.a",
				typ: "x", subtype: "y", action: "z.a"},
			{name: "type-subtype", inputType: "x.y.z", inputSubtype: "a",
				typ: "x.y.z", subtype: "a"},
			{name: "type-action", inputType: "x.y.z", inputAction: "b",
				typ: "x.y.z", action: "b"},
			{name: "type-subtype-action", inputType: "x.y", inputSubtype: "a", inputAction: "b",
				typ: "x.y", subtype: "a", action: "b"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var input span
				modeldecodertest.SetStructValues(&input, "init", 5000, true, time.Now())
				input.Type.Set(tc.inputType)
				if tc.inputSubtype != "" {
					input.Subtype.Set(tc.inputSubtype)
				} else {
					input.Subtype.Reset()
				}
				if tc.inputAction != "" {
					input.Action.Set(tc.inputAction)
				} else {
					input.Action.Reset()
				}
				var out model.Span
				mapToSpanModel(&input, initializedMeta(), time.Now(), modeldecoder.Config{}, &out)
				assert.Equal(t, tc.typ, out.Type)
				if tc.subtype == "" {
					assert.Nil(t, out.Subtype)
				} else {
					require.NotNil(t, out.Subtype)
					assert.Equal(t, tc.subtype, *out.Subtype)
				}
				if tc.action == "" {
					assert.Nil(t, out.Action)
				} else {
					require.NotNil(t, out.Action)
					assert.Equal(t, tc.action, *out.Action)
				}
			})
		}
	})
}
