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
	"net/http"
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
	inp := `{"error":{"id":"tr-a"}}`
	root := fetchErrorRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseErrorRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedError(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: true}}
		str := `{"error":{"id":"a-b-c","timestamp":1599996822281000,"log":{"message":"abc"},"context":{"experimental":"exp"}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Error
		require.NoError(t, DecodeNestedError(dec, &input, &out))
		assert.Equal(t, "exp", out.Experimental)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		str = `{"error":{"id":"a-b-c","log":{"message":"abc"},"context":{"experimental":"exp"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		out = model.Error{}
		require.NoError(t, DecodeNestedError(dec, &input, &out))
		// experimental should only be set if allowed by configuration
		assert.Nil(t, out.Experimental)
		// if no timestamp is provided, fall back to request time
		assert.Equal(t, now, out.Timestamp)

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
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")
	exceptions := func(key string) bool { return false }

	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero event values
		var input errorEvent
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		// iterate through metadata model and assert values are set
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, defaultVal)
	})

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input errorEvent
		var out model.Error
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: true}, &out)
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
		modeldecodertest.AssertStructValues(t, &out.Metadata.Service, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out.Metadata.User, exceptions, otherVal)
	})

	t.Run("client-ip-header", func(t *testing.T) {
		var input errorEvent
		var out model.Error
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToErrorModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, gatewayIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input errorEvent
		var out model.Error
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToErrorModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, randomIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
	})

	t.Run("error-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// metadata are tested separately
				"Metadata",
				// URL parts are derived from url (separately tested)
				"Page.URL",
				// exception.parent is only set after calling `flattenExceptionTree` (not part of decoding)
				"Exception.Parent",
				// GroupingKey is set by a model processor
				"GroupingKey",
				// HTTP headers tested in 'http-headers'
				"HTTP.Request.Headers",
				"HTTP.Response.Headers",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"Exception.Stacktrace.Original",
				"Exception.Stacktrace.Sourcemap",
				"Log.Stacktrace.Original",
				"Log.Stacktrace.Sourcemap",
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
		mapToErrorModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// set Timestamp to requestTime if eventTime is zero
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToErrorModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// reuse input model for different event
		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out2)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input errorEvent
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b"}, "c": []string{"d", "e"}}, out.HTTP.Request.Headers)
		assert.Equal(t, common.MapStr{"f": []string{"g"}}, out.HTTP.Response.Headers)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Page.URL.Full)
		assert.Equal(t, "https://my.site.test:9201", out.URL.Full)
		assert.Equal(t, 9201, out.Page.URL.Port)
		assert.Equal(t, "https", out.Page.URL.Scheme)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out model.Error
		mapToErrorModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Page.Referer)
		assert.Equal(t, "https://my.site.test:9201", out.HTTP.Request.Referrer)
	})
}
