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
		eventBase := initializedMetadata()
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"e":{"id":"a-b-c","timestamp":1599996822281000,"log":{"mg":"abc"}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch model.Batch
		require.NoError(t, DecodeNestedError(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Error)
		defaultValues := modeldecodertest.DefaultValues()
		defaultValues.Update(time.Unix(1599996822, 281000000).UTC())
		modeldecodertest.AssertStructValues(t, &batch[0], metadataExceptions(), defaultValues)

		// if no timestamp is provided, leave base event timestamp unmodified
		input = modeldecoder.Input{Base: eventBase}
		str = `{"e":{"id":"a-b-c","log":{"mg":"abc"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = model.Batch{}
		require.NoError(t, DecodeNestedError(dec, &input, &batch))
		assert.Equal(t, now, batch[0].Timestamp)

		// test decode
		err := DecodeNestedError(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedError(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToErrorModel(t *testing.T) {
	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input errorEvent
		out := initializedMetadata()
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, &out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Client.IP, out.Client.IP.String())
		assert.Equal(t, common.MapStr{
			"init0": "init", "init1": "init", "init2": "init",
			"overwritten0": "overwritten", "overwritten1": "overwritten",
		}, out.Labels)
		// service and user values should be set
		modeldecodertest.AssertStructValues(t, &out.Service, metadataExceptions("Node", "Agent.EphemeralID"), otherVal)
		modeldecodertest.AssertStructValues(t, &out.User, metadataExceptions(), otherVal)
	})

	t.Run("error-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// GroupingKey is set by a model processor
				"GroupingKey",
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
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToErrorModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Error, exceptions, defaultVal)

		// leave event timestamp unmodified if eventTime is zero
		defaultVal.Update(time.Time{})
		out1.Timestamp = reqTime
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToErrorModel(&input, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Error, exceptions, defaultVal)

		// reuse input model for different event
		// ensure memory is not shared by reusing input model
		out2.Timestamp = reqTime
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToErrorModel(&input, &out2)
		modeldecodertest.AssertStructValues(t, out2.Error, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, out1.Error, exceptions, defaultVal)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToErrorModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.URL.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToErrorModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.HTTP.Request.Referrer)
	})

	t.Run("loggerName", func(t *testing.T) {
		var input errorEvent
		input.Log.Message.Set("log message")
		var out model.APMEvent
		mapToErrorModel(&input, &out)
		require.NotNil(t, out.Error.Log.LoggerName)
		assert.Equal(t, "default", out.Error.Log.LoggerName)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input errorEvent
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.APMEvent
		mapToErrorModel(&input, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b"}, "c": []string{"d", "e"}}, out.HTTP.Request.Headers)
		assert.Equal(t, common.MapStr{"f": []string{"g"}}, out.HTTP.Response.Headers)
	})

	t.Run("exception-code", func(t *testing.T) {
		var input errorEvent
		var out model.APMEvent
		input.Exception.Code.Set(123.456)
		mapToErrorModel(&input, &out)
		assert.Equal(t, "123", out.Error.Exception.Code)
	})
}
