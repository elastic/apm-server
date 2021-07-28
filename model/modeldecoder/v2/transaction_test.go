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
	"encoding/json"
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

func TestResetTransactionOnRelease(t *testing.T) {
	inp := `{"transaction":{"name":"tr-a"}}`
	root := fetchTransactionRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseTransactionRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: true}}
		str := `{"transaction":{"duration":100,"timestamp":1599996822281000,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"context":{"experimental":"exp"}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))

		var batch model.Batch
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		require.Len(t, batch, 1)
		require.NotNil(t, batch[0].Transaction)
		assert.Equal(t, "request", batch[0].Transaction.Type)
		assert.Equal(t, "exp", batch[0].Transaction.Experimental)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", batch[0].Transaction.Timestamp.String())

		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"context":{"experimental":"exp"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = model.Batch{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &batch))
		// experimental should only be set if allowed by configuration
		assert.Nil(t, batch[0].Transaction.Experimental)
		// if no timestamp is provided, fall back to request time
		assert.Equal(t, now, batch[0].Transaction.Timestamp)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")
	exceptions := func(key string) bool {
		return key == "RepresentativeCount" || strings.HasPrefix(key, "System.Network")
	}

	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero event values
		var input transaction
		var out model.APMEvent
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: true}, &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Transaction.Metadata, exceptions, modeldecodertest.DefaultValues())
	})

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input transaction
		var out model.APMEvent
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: true}, &out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.Transaction.Metadata.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Transaction.Metadata.Client.IP, out.Transaction.Metadata.Client.IP.String())
		// metadata labels and event labels should not be merged
		mLabels := common.MapStr{"init0": "init", "init1": "init", "init2": "init"}
		tLabels := common.MapStr{"overwritten0": "overwritten", "overwritten1": "overwritten"}
		assert.Equal(t, mLabels, out.Transaction.Metadata.Labels)
		assert.Equal(t, tLabels, out.Transaction.Labels)
		// service and user values should be set
		modeldecodertest.AssertStructValues(t, &out.Transaction.Metadata.Service, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out.Transaction.Metadata.User, exceptions, otherVal)
	})

	t.Run("client-ip-header", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		// from headers (case insensitive)
		input.Context.Request.Headers.Val.Add("x-Real-ip", gatewayIP.String())
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, gatewayIP.String(), out.Transaction.Metadata.Client.IP.String())
		// ignore if set in metadata
		out = model.APMEvent{}
		metadata := model.Metadata{Client: model.Client{IP: net.ParseIP("192.17.1.1")}}
		mapToTransactionModel(&input, metadata, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "192.17.1.1", out.Transaction.Metadata.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		// set invalid headers
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-Real-ip", "192.13.14:8097")
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		// ensure client ip is populated from socket
		assert.Equal(t, randomIP.String(), out.Transaction.Metadata.Client.IP.String())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		var out model.APMEvent
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "test@user.com", out.Transaction.Metadata.User.Email)
		assert.Zero(t, out.Transaction.Metadata.User.ID)
		assert.Zero(t, out.Transaction.Metadata.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// All the below exceptions are tested separately
			if strings.HasPrefix(key, "Metadata") || strings.HasPrefix(key, "Page.URL") {
				return true
			}
			switch key {
			case "Headers", "Experimental", "RepresentativeCount":
				return true
			}
			return true
		}

		var input transaction
		var out1, out2 model.APMEvent
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out1)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// set Timestamp to requestTime if eventTime is zero
		defaultVal.Update(time.Time{})
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out1)
		defaultVal.Update(reqTime)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{Experimental: true}, &out2)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input transaction
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.APMEvent
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, common.MapStr{"a": []string{"b"}, "c": []string{"d", "e"}}, out.Transaction.HTTP.Request.Headers)
		assert.Equal(t, common.MapStr{"f": []string{"g"}}, out.Transaction.HTTP.Response.Headers)
	})

	t.Run("http-request-body", func(t *testing.T) {
		var input transaction
		input.Context.Request.Body.Set(map[string]interface{}{
			"a": json.Number("123.456"),
			"b": nil,
			"c": "d",
		})
		var out model.APMEvent
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, map[string]interface{}{"a": common.Float(123.456), "c": "d"}, out.Transaction.HTTP.Request.Body)
	})

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Transaction.Page.URL.Full)
		assert.Equal(t, "https://my.site.test:9201", out.Transaction.URL.Full)
		assert.Equal(t, 9201, out.Transaction.Page.URL.Port)
		assert.Equal(t, "https", out.Transaction.Page.URL.Scheme)
	})

	t.Run("page.referer", func(t *testing.T) {
		var input transaction
		input.Context.Page.Referer.Set("https://my.site.test:9201")
		var out model.APMEvent
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Transaction.Page.Referer)
		assert.Equal(t, "https://my.site.test:9201", out.Transaction.HTTP.Request.Referrer)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 4.0, out.Transaction.RepresentativeCount)
		// sample rate is not set -> Representative Count should be 1 by default
		out.Transaction.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Reset()
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 1.0, out.Transaction.RepresentativeCount)
		// sample rate is set to 0
		out.Transaction.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Set(0)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 0.0, out.Transaction.RepresentativeCount)
	})

	t.Run("outcome", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "failure", out.Transaction.Outcome)
		// derive from other fields - success
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "success", out.Transaction.Outcome)
		// derive from other fields - failure
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusInternalServerError)
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "failure", out.Transaction.Outcome)
		// derive from other fields - unknown
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "unknown", out.Transaction.Outcome)
	})

	t.Run("session", func(t *testing.T) {
		var input transaction
		var out model.APMEvent
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		input.Session.ID.Reset()
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, model.TransactionSession{}, out.Transaction.Session)

		input.Session.ID.Set("session_id")
		input.Session.Sequence.Set(123)
		mapToTransactionModel(&input, model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, model.TransactionSession{
			ID:       "session_id",
			Sequence: 123,
		}, out.Transaction.Session)
	})
}
