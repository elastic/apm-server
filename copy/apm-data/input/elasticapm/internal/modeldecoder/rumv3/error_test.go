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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecodertest"
	"github.com/elastic/apm-data/model/modelpb"
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
		now := modelpb.FromTime(time.Now())
		eventBase := initializedMetadata()
		eventBase.Timestamp = now
		input := modeldecoder.Input{Base: eventBase}
		str := `{"e":{"id":"a-b-c","timestamp":1599996822281000,"log":{"mg":"abc"}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var batch modelpb.Batch
		require.NoError(t, DecodeNestedError(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Error)
		assert.Equal(t, modelpb.FromTime(time.Unix(1599996822, 281000000)), batch[0].Timestamp)
		assert.Empty(t, cmp.Diff(&modelpb.Error{
			Id: "a-b-c",
			Log: &modelpb.ErrorLog{
				Message:    "abc",
				LoggerName: "default",
			},
		}, batch[0].Error, protocmp.Transform()))

		// if no timestamp is provided, leave base event timestamp unmodified
		input = modeldecoder.Input{Base: eventBase}
		str = `{"e":{"id":"a-b-c","log":{"mg":"abc"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = modelpb.Batch{}
		require.NoError(t, DecodeNestedError(dec, &input, &batch))
		assert.Equal(t, now, batch[0].Timestamp)

		// test decode
		err := DecodeNestedError(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch modelpb.Batch
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
		input.Exception.Cause = nil
		mapToErrorModel(&input, out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Client.Ip)
		assert.Equal(t, modelpb.Labels{
			"init0": {Global: true, Value: "init"}, "init1": {Global: true, Value: "init"}, "init2": {Global: true, Value: "init"},
			"overwritten0": {Value: "overwritten"}, "overwritten1": {Value: "overwritten"},
		}, modelpb.Labels(out.Labels))
		// service and user values should be set
		modeldecodertest.AssertStructValues(t, &out.Service, metadataExceptions("node", "Agent.EphemeralID"), otherVal)
		modeldecodertest.AssertStructValues(t, &out.User, metadataExceptions(), otherVal)
	})

	t.Run("error-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// GroupingKey is set by a model processor
				"grouping_key",
				// StackTrace is only set by processor/otel
				"stack_trace",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"exception.stacktrace.original",
				"exception.stacktrace.sourcemap",
				"log.stacktrace.original",
				"log.stacktrace.sourcemap",
				// not set by rumv3
				"exception.stacktrace.vars",
				"log.stacktrace.vars",
				"exception.stacktrace.library_frame",
				"log.stacktrace.library_frame",
				// ExcludeFromGrouping is set when processing the event
				"exception.stacktrace.exclude_from_grouping",
				"log.stacktrace.exclude_from_grouping",
				// Message and Type are only set for ECS compatible log event type
				"message",
				"type",
				// populator adding invalid values
				"exception.cause",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}
		var input errorEvent
		var out1, out2 modelpb.APMEvent
		reqTime := modelpb.FromTime(time.Now().Add(time.Second))
		out1.Timestamp = reqTime
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Exception.Cause = nil
		mapToErrorModel(&input, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, out1.Error, exceptions, defaultVal)

		// leave event timestamp unmodified if eventTime is zero
		out1.Timestamp = reqTime
		modeldecodertest.SetStructValues(&input, defaultVal)
		input.Exception.Cause = nil
		mapToErrorModel(&input, &out1)
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
		var out modelpb.APMEvent
		mapToErrorModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Url.Full)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input errorEvent
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out modelpb.APMEvent
		mapToErrorModel(&input, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Http.Request.Referrer)
	})

	t.Run("loggerName", func(t *testing.T) {
		var input errorEvent
		input.Log.Message.Set("log message")
		var out modelpb.APMEvent
		mapToErrorModel(&input, &out)
		require.NotNil(t, out.Error.Log.LoggerName)
		assert.Equal(t, "default", out.Error.Log.LoggerName)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input errorEvent
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out modelpb.APMEvent
		mapToErrorModel(&input, &out)
		assert.Empty(t, cmp.Diff([]*modelpb.HTTPHeader{
			{
				Key:   "a",
				Value: []string{"b"},
			},
			{
				Key:   "c",
				Value: []string{"d", "e"},
			},
		}, out.Http.Request.Headers,
			cmpopts.SortSlices(func(x, y *modelpb.HTTPHeader) bool {
				return x.Key < y.Key
			}),
			protocmp.Transform(),
		))

		assert.Equal(t, []*modelpb.HTTPHeader{
			{
				Key:   "f",
				Value: []string{"g"},
			},
		}, out.Http.Response.Headers)
	})

	t.Run("exception-code", func(t *testing.T) {
		var input errorEvent
		var out modelpb.APMEvent
		input.Exception.Code.Set(123.456)
		mapToErrorModel(&input, &out)
		assert.Equal(t, "123", out.Error.Exception.Code)
	})

	t.Run("transaction-name", func(t *testing.T) {
		var input errorEvent
		var out modelpb.APMEvent
		input.Transaction.Name.Set("My Transaction")
		mapToErrorModel(&input, &out)
		assert.Equal(t, "My Transaction", out.Transaction.Name)
	})
}
