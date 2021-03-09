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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestResetErrorOnRelease(t *testing.T) {
	inp := `{"e":{"id":"tr-a"}}`
	root := fetchErrorRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseErrorRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedError(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now}
		str := `{"e":{"id":"a-b-c","timestamp":1599996822281000,"log":{"mg":"abc"}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Error
		require.NoError(t, DecodeNestedError(dec, &input, &out))
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		// if no timestamp is provided, fall back to request time
		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now}
		str = `{"e":{"id":"a-b-c","log":{"mg":"abc"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		out = model.Error{}
		require.NoError(t, DecodeNestedError(dec, &input, &out))
		assert.Equal(t, now, out.Timestamp)

		// test decode
		err := DecodeNestedError(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out model.Error
		err := DecodeNestedError(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToErrorModel(t *testing.T) {
	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero event values
		var input errorEvent
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), &out)
		// iterate through metadata model and assert values are set
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.AssertStructValues(t, &out.Metadata, metadataExceptions(), defaultVal)
	})

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input errorEvent
		var out model.Error
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, initializedMetadata(), time.Now(), &out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.Metadata.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
		// metadata labels and event labels should not be merged
		mLabels := common.MapStr{"init0": "init", "init1": "init", "init2": "init"}
		tLabels := common.MapStr{"overwritten0": "overwritten", "overwritten1": "overwritten"}
		assert.Equal(t, mLabels, out.Metadata.Labels)
		assert.Equal(t, tLabels, out.Labels)
		// service and user values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.Service, metadataExceptions("Node", "Agent.EphemeralID"), otherVal)
		modeldecodertest.AssertStructValues(t, &out.Metadata.User, metadataExceptions(), otherVal)
	})

	t.Run("error-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// metadata are tested separately
				"Metadata",
				// values not set for RUM v3
				"HTTP.Request.Env", "HTTP.Request.Body", "HTTP.Request.Socket", "HTTP.Request.Cookies",
				"HTTP.Response.HeadersSent", "HTTP.Response.Finished",
				"Experimental",
				// URL parts are derived from url (separately tested)
				"URL", "Page.URL",
				// RUM is set in stream processor
				"RUM",
				// exception.parent is only set after calling `flattenExceptionTree` (not part of decoding)
				"Exception.Parent",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"Exception.Stacktrace.Original",
				"Exception.Stacktrace.Sourcemap",
				"Log.Stacktrace.Original",
				"Log.Stacktrace.Sourcemap",
				// not set by rumv3
				"Exception.Stacktrace.Vars",
				"Log.Stacktrace.Vars",
				"Exception.Stacktrace.LibraryFrame",
				"Log.Stacktrace.LibraryFrame",
				// ExcludeFromGrouping is set when processing the event
				"Exception.Stacktrace.ExcludeFromGrouping",
				"Log.Stacktrace.ExcludeFromGrouping"} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}
		var input errorEvent
		var out1, out2 model.Error
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToErrorModel(&input, initializedMetadata(), reqTime, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// set Timestamp to requestTime if eventTime is zero
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToErrorModel(&input, initializedMetadata(), reqTime, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// reuse input model for different event
		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, initializedMetadata(), reqTime, &out2)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), &out)
		assert.Equal(t, "https://my.site.test:9201", out.Page.URL.Full)
		assert.Equal(t, "https://my.site.test:9201", out.URL.Full)
		assert.Equal(t, 9201, out.Page.URL.Port)
		assert.Equal(t, "https", out.Page.URL.Scheme)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), &out)
		assert.Equal(t, "https://my.site.test:9201", out.Page.Referer)
		assert.Equal(t, "https://my.site.test:9201", out.HTTP.Request.Referer)
	})

	t.Run("loggerName", func(t *testing.T) {
		var input errorEvent
		input.Log.Message.Set("log message")
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), &out)
		require.NotNil(t, out.Log.LoggerName)
		assert.Equal(t, "default", out.Log.LoggerName)
	})
}
