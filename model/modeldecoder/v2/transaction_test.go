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
		var out model.Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Type)
		assert.Equal(t, "exp", out.Experimental)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"context":{"experimental":"exp"}}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		out = model.Transaction{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		// experimental should only be set if allowed by configuration
		assert.Nil(t, out.Experimental)
		// if no timestamp is provided, fall back to request time
		assert.Equal(t, now, out.Timestamp)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out model.Transaction
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")
	exceptions := func(key string) bool {
		return key == "RepresentativeCount"
	}

	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero event values
		var input transaction
		var out model.Transaction
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: true}, &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, modeldecodertest.DefaultValues())
	})

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input transaction
		var out model.Transaction
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: true}, &out)
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
		var input transaction
		var out model.Transaction
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		// from headers (case insensitive)
		input.Context.Request.Headers.Val.Add("x-Real-ip", gatewayIP.String())
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, gatewayIP.String(), out.Metadata.Client.IP.String())
		// ignore if set in metadata
		out = model.Transaction{}
		metadata := model.Metadata{Client: model.Client{IP: net.ParseIP("192.17.1.1")}}
		mapToTransactionModel(&input, &metadata, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "192.17.1.1", out.Metadata.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input transaction
		var out model.Transaction
		// set invalid headers
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-Real-ip", "192.13.14:8097")
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{Experimental: false}, &out)
		// ensure client ip is populated from socket
		assert.Equal(t, randomIP.String(), out.Metadata.Client.IP.String())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		var out model.Transaction
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "test@user.com", out.Metadata.User.Email)
		assert.Zero(t, out.Metadata.User.ID)
		assert.Zero(t, out.Metadata.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// metadata are tested separately
			if strings.HasPrefix(key, "Metadata") ||
				// URL parts are derived from url (separately tested)
				strings.HasPrefix(key, "Page.URL") ||
				// experimental is tested separately
				key == "Experimental" ||
				// RepresentativeCount is not set by decoder
				key == "RepresentativeCount" {
				return true
			}
			return false
		}

		var input transaction
		var out1, out2 model.Transaction
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

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.Transaction
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{Experimental: false}, &out)
		assert.Equal(t, "https://my.site.test:9201", out.Page.URL.Full)
		assert.Equal(t, 9201, out.Page.URL.Port)
		assert.Equal(t, "https", out.Page.URL.Scheme)
	})

	t.Run("sample-rate", func(t *testing.T) {
		var input transaction
		var out model.Transaction
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// sample rate is set to > 0
		input.SampleRate.Set(0.25)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 4.0, out.RepresentativeCount)
		// sample rate is not set -> Representative Count should be 1 by default
		out.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Reset()
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 1.0, out.RepresentativeCount)
		// sample rate is set to 0
		out.RepresentativeCount = 0.0 //reset to zero value
		input.SampleRate.Set(0)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, 0.0, out.RepresentativeCount)
	})

	t.Run("outcome", func(t *testing.T) {
		var input transaction
		var out model.Transaction
		modeldecodertest.SetStructValues(&input, modeldecodertest.DefaultValues())
		// set from input, ignore status code
		input.Outcome.Set("failure")
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "failure", out.Outcome)
		// derive from other fields - success
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusBadRequest)
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "success", out.Outcome)
		// derive from other fields - failure
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Set(http.StatusInternalServerError)
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "failure", out.Outcome)
		// derive from other fields - unknown
		input.Outcome.Reset()
		input.Context.Response.StatusCode.Reset()
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), modeldecoder.Config{}, &out)
		assert.Equal(t, "unknown", out.Outcome)
	})
}
