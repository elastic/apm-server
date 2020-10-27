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
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now}
		str := `{"x":{"n":"tr-a","d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"y":[{"n":"a","d":10,"t":"http","id":"123","s":20}],"me":[{"sa":{"xds":{"v":2048}}},{"sa":{"ysc":{"v":5}}}]}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Transaction.Type)
		// fall back to request time
		assert.Equal(t, now, out.Transaction.Timestamp)
		// ensure nested metricsets are decoded
		require.Equal(t, 2, len(out.Metricsets))
		assert.Equal(t, []model.Sample{{Name: "transaction.duration.sum.us", Value: 2048}}, out.Metricsets[0].Samples)
		m := out.Metricsets[1]
		assert.Equal(t, []model.Sample{{Name: "span.self_time.count", Value: 5}}, m.Samples)
		assert.Equal(t, "tr-a", m.Transaction.Name)
		assert.Equal(t, "request", m.Transaction.Type)
		assert.Equal(t, now, m.Timestamp)
		// ensure nested spans are decoded
		require.Equal(t, 1, len(out.Spans))
		sp := out.Spans[0]
		start := time.Duration(20 * 1000 * 1000)
		assert.Equal(t, now.Add(start), sp.Timestamp) //add start to timestamp
		assert.Equal(t, "100", sp.TransactionID)
		assert.Equal(t, "1", sp.TraceID)
		assert.Equal(t, "100", sp.ParentID)

		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var out Transaction
		err := DecodeNestedTransaction(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToTransactionModel(t *testing.T) {
	localhostIP := net.ParseIP("127.0.0.1")

	t.Run("metadata-set", func(t *testing.T) {
		// do not overwrite metadata with zero transaction values
		var input transaction
		var out model.Transaction
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), &out)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &out.Metadata, metadataExceptions(), modeldecodertest.DefaultValues())
	})

	//TODO(simitt):test metadata are not sharing memory between events (nested labels!)
	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var input transaction
		var out model.Transaction
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), time.Now(), &out)

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
		modeldecodertest.AssertStructValues(t, &out.Metadata.Service, metadataExceptions("Node", "Agent.EphemeralID"), otherVal)
		// user values should be set
		modeldecodertest.AssertStructValues(t, &out.Metadata.User, metadataExceptions(), otherVal)
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&inputTr, initializedMetadata(), time.Now(), &tr)
		assert.Equal(t, "test@user.com", tr.Metadata.User.Email)
		assert.Zero(t, tr.Metadata.User.ID)
		assert.Zero(t, tr.Metadata.User.Name)
	})

	t.Run("transaction-values", func(t *testing.T) {
		//TODO(simitt): add tests for spans
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
		mapToTransactionModel(&input, initializedMetadata(), reqTime, &out1)
		input.Reset()
		defaultVal.Update(reqTime) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		otherVal.Update(reqTime) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToTransactionModel(&input, initializedMetadata(), reqTime, &out2)
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("span-values", func(t *testing.T) {
		exceptions := func(key string) bool {
			for _, s := range []string{
				// metadata are tested separately
				"Metadata",
				// values not set for RUM v3
				"ChildIDs",
				"DB",
				"Experimental",
				"HTTP.Response.Headers",
				"Message",
				"RepresentativeCount",
				"Service.Agent.EphemeralID",
				"Service.Environment",
				"Service.Framework",
				"Service.Language",
				"Service.Node",
				"Service.Runtime",
				"Service.Version",
				"Stacktrace.LibraryFrame",
				"Stacktrace.Vars",
				// set as HTTP.StatusCode for RUM v3
				"HTTP.Response.StatusCode",
				// stacktrace original and sourcemap values are set when sourcemapping is applied
				"Stacktrace.Original",
				"Stacktrace.Sourcemap",
				// ExcludeFromGrouping is set when processing the event
				"Stacktrace.ExcludeFromGrouping",
				// RUM is set in stream processor
				"RUM",
				// Transaction related information is set within the DecodeNestedTransaction method
				// it is separatly tested in TestDecodeNestedTransaction
				"TransactionID", "TraceID", "ParentID",
			} {
				if strings.HasPrefix(key, s) {
					return true
				}
			}
			return false
		}

		var input span
		var out1, out2 model.Span
		reqTime := time.Now().Add(time.Second)
		defaultVal := modeldecodertest.DefaultValues()
		modeldecodertest.SetStructValues(&input, defaultVal)
		mapToSpanModel(&input, initializedMetadata(), reqTime, &out1)
		input.Reset()
		defaultStart := time.Duration(defaultVal.Float * 1000 * 1000)
		defaultVal.Update(reqTime.Add(defaultStart)) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)

		// ensure memory is not shared by reusing input model
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToSpanModel(&input, initializedMetadata(), reqTime, &out2)
		otherStart := time.Duration(otherVal.Float * 1000 * 1000)
		otherVal.Update(reqTime.Add(otherStart)) //for rumv3 the timestamp is always set from the request time
		modeldecodertest.AssertStructValues(t, &out2, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out1, exceptions, defaultVal)
	})

	t.Run("page.URL", func(t *testing.T) {
		var inputTr transaction
		inputTr.Context.Page.URL.Set("https://my.site.test:9201")
		var tr model.Transaction
		mapToTransactionModel(&inputTr, initializedMetadata(), time.Now(), &tr)
		assert.Equal(t, "https://my.site.test:9201", *tr.Page.URL.Full)
		assert.Equal(t, 9201, *tr.Page.URL.Port)
		assert.Equal(t, "https", *tr.Page.URL.Scheme)
	})

}
