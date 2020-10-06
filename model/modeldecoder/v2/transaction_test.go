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
	require.NoError(t, decoder.NewJSONIteratorDecoder(strings.NewReader(inp)).Decode(root))
	require.True(t, root.IsSet())
	releaseTransactionRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: true}}
		str := `{"transaction":{"duration":100,"timestamp":1599996822281000,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"experimental":"exp"}}`
		dec := decoder.NewJSONIteratorDecoder(strings.NewReader(str))
		var out model.Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Type)
		assert.Equal(t, "exp", out.Experimental)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"experimental":"exp"}}`
		dec = decoder.NewJSONIteratorDecoder(strings.NewReader(str))
		out = model.Transaction{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		// experimental should only be set if allowed by configuration
		assert.Nil(t, out.Experimental)
		// if no timestamp is provided, fall back to request time
		assert.Equal(t, now, out.Timestamp)

		err := DecodeNestedTransaction(decoder.NewJSONIteratorDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out model.Transaction
		err := DecodeNestedTransaction(decoder.NewJSONIteratorDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	localhostIP := net.ParseIP("127.0.0.1")
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")
	exceptions := func(key string) bool {
		return key == "RepresentativeCount"
	}

	initializedMeta := func() *model.Metadata {
		var inputMeta metadata
		var meta model.Metadata
		modeldecodertest.SetStructValues(&inputMeta, "meta", 1, false, time.Now())
		mapToMetadataModel(&inputMeta, &meta)
		// initialize values that are not set by input
		meta.UserAgent = model.UserAgent{Name: "meta", Original: "meta"}
		meta.Client.IP = localhostIP
		meta.System.IP = localhostIP
		return &meta
	}

	t.Run("set-metadata", func(t *testing.T) {
		// do not overwrite metadata with zero transaction values
		var input transaction
		var out model.Transaction
		mapToTransactionModel(&input, initializedMeta(), time.Now(), true, &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, "meta", 1, false, localhostIP, time.Now())
	})

	t.Run("overwrite-metadata", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var input transaction
		var out model.Transaction
		modeldecodertest.SetStructValues(&input, "overwritten", 5000, false, time.Now())
		input.Context.Request.Headers.Val.Add("user-agent", "first")
		input.Context.Request.Headers.Val.Add("user-agent", "second")
		input.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		mapToTransactionModel(&input, initializedMeta(), time.Now(), true, &out)

		// user-agent should be set to context request header values
		assert.Equal(t, "first, second", out.Metadata.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		assert.Equal(t, localhostIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
		// metadata labels and transaction labels should not be merged
		assert.Equal(t, common.MapStr{"meta": "meta"}, out.Metadata.Labels)
		assert.Equal(t, &model.Labels{"overwritten": "overwritten"}, out.Labels)
		// service values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.Service, exceptions, "overwritten", 100, true, localhostIP, time.Now())
		// user values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.User, exceptions, "overwritten", 100, true, localhostIP, time.Now())
	})

	t.Run("client-ip-header", func(t *testing.T) {
		var input transaction
		var out model.Transaction
		input.Context.Request.Headers.Set(http.Header{})
		input.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), false, &out)
		assert.Equal(t, gatewayIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var input transaction
		var out model.Transaction
		input.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&input, &model.Metadata{}, time.Now(), false, &out)
		assert.Equal(t, randomIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input transaction
		var out model.Transaction
		input.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&input, initializedMeta(), time.Now(), false, &out)
		assert.Equal(t, "test@user.com", out.Metadata.User.Email)
		assert.Zero(t, out.Metadata.User.ID)
		assert.Zero(t, out.Metadata.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			// metadata are tested separately
			// URL parts are derived from url (separately tested)
			// experimental is tested separately
			// RepresentativeCount is not set by decoder
			if strings.HasPrefix(key, "Metadata") || strings.HasPrefix(key, "Page.URL") ||
				key == "Experimental" || key == "RepresentativeCount" {
				return true
			}

			return false
		}

		var input transaction
		var out model.Transaction
		eventTime, reqTime := time.Now(), time.Now().Add(time.Second)
		modeldecodertest.SetStructValues(&input, "overwritten", 5000, true, eventTime)
		mapToTransactionModel(&input, initializedMeta(), reqTime, true, &out)
		modeldecodertest.AssertStructValues(t, &out, exceptions, "overwritten", 5000, true, localhostIP, eventTime)

		// set requestTime if eventTime is zero
		modeldecodertest.SetStructValues(&input, "overwritten", 5000, true, time.Time{})
		out = model.Transaction{}
		mapToTransactionModel(&input, initializedMeta(), reqTime, true, &out)
		input.Reset()
		modeldecodertest.AssertStructValues(t, &out, exceptions, "overwritten", 5000, true, localhostIP, reqTime)

	})

	t.Run("page.URL", func(t *testing.T) {
		var input transaction
		input.Context.Page.URL.Set("https://my.site.test:9201")
		var out model.Transaction
		mapToTransactionModel(&input, initializedMeta(), time.Now(), false, &out)
		assert.Equal(t, "https://my.site.test:9201", *out.Page.URL.Full)
		assert.Equal(t, 9201, *out.Page.URL.Port)
		assert.Equal(t, "https", *out.Page.URL.Scheme)
	})

}
