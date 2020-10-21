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
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestResetTransactionOnRelease(t *testing.T) {
	inp := `{"x":{"n":"tr-a"}}`
	tr := fetchTransactionRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(tr))
	require.True(t, tr.IsSet())
	releaseTransactionRoot(tr)
	assert.False(t, tr.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{}}
		str := `{"x":{"d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2}}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Type)
		// fall back to request time
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
	localhostIP := net.ParseIP("127.0.0.1")
	exceptions := func(key string) bool {
		// values not set for rumV3:
		for _, k := range []string{"Cloud", "System", "Process", "Service.Node", "Node"} {
			if strings.HasPrefix(key, k) {
				return true
			}
		}
		for _, k := range []string{"Service.Agent.EphemeralID",
			"Agent.EphemeralID", "Message", "RepresentativeCount"} {
			if k == key {
				return true
			}
		}
		return false
	}

	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero transaction values
		var input transaction
		var out model.Transaction
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Metadata, exceptions, modeldecodertest.DefaultValues())
	})

	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var input transaction
		var out model.Transaction
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), modeldecoder.Config{}, &out)

		// user-agent should be set to context request header values
		assert.Equal(t, "d, e", out.Metadata.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		assert.Equal(t, localhostIP, out.Metadata.Client.IP, out.Metadata.Client.IP.String())
		// metadata labels and transaction labels should not be merged
		mLabels := common.MapStr{"init0": "init", "init1": "init", "init2": "init"}
		assert.Equal(t, mLabels, out.Metadata.Labels)
		tLabels := model.Labels{"overwritten0": "overwritten", "overwritten1": "overwritten"}
		assert.Equal(t, &tLabels, out.Labels)
		// service values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.Service, exceptions, otherVal)
		// user values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.User, exceptions, otherVal)
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&inputTr, initializedMetadata(), time.Now(), modeldecoder.Config{}, &tr)
		assert.Equal(t, "test@user.com", tr.Metadata.User.Email)
		assert.Zero(t, tr.Metadata.User.ID)
		assert.Zero(t, tr.Metadata.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		exceptions := func(key string) bool {

			for _, s := range []string{
				// metadata are tested separately
				"Metadata",
				// values not set for RUM v3
				"HTTP.Request.Env", "HTTP.Request.Body", "HTTP.Request.Socket", "HTTP.Request.Cookies",
				"HTTP.Response.HeadersSent", "HTTP.Response.Finished",
				"Experimental",
				"RepresentativeCount", "Message",
				// URL parts are derived from url (separately tested)
				"URL", "Page.URL",
				// RUM is set in stream processor
				"RUM",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input transaction
		var out1, out2 model.Transaction
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{}, &out1)
		input.Reset()
		defaultVal.Update(reqTime) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		otherVal.Update(reqTime) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, modeldecoder.Config{}, &out2)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("page.URL", func(t *testing.T) {
		var inputTr transaction
		inputTr.Context.Page.URL.Set("https://my.site.test:9201")
		var tr model.Transaction
		mapToTransactionModel(&inputTr, initializedMetadata(), time.Now(), modeldecoder.Config{}, &tr)
		assert.Equal(t, "https://my.site.test:9201", *tr.Page.URL.Full)
		assert.Equal(t, 9201, *tr.Page.URL.Port)
		assert.Equal(t, "https", *tr.Page.URL.Scheme)
	})

}
