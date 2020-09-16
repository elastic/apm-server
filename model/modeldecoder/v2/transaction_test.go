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

func TestTransactionSetResetIsSet(t *testing.T) {
	var tRoot transactionRoot
	modeldecodertest.DecodeData(t, reader(t, "transactions"), "transaction", &tRoot)
	require.True(t, tRoot.IsSet())
	// call Reset and ensure initial state, except for array capacity
	tRoot.Reset()
	assert.False(t, tRoot.IsSet())
}

func TestResetTransactionOnRelease(t *testing.T) {
	inp := `{"transaction":{"name":"tr-a"}}`
	tr := fetchTransactionRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(tr))
	require.True(t, tr.IsSet())
	releaseTransactionRoot(tr)
	assert.False(t, tr.IsSet())
}

func TestDecodeNestedTransaction(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: true}}
		str := `{"transaction":{"duration":100,"timestamp":1599996822281000,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"experimental":"exp"}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Type)
		assert.Equal(t, "exp", out.Experimental)
		assert.Equal(t, "2020-09-13 11:33:42.281 +0000 UTC", out.Timestamp.String())

		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		str = `{"transaction":{"duration":100,"id":"100","trace_id":"1","type":"request","span_count":{"started":2},"experimental":"exp"}}`
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
		var inputTr transaction
		var tr model.Transaction
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), true, &tr)
		// iterate through metadata model and assert values are set
		modeldecodertest.AssertStructValues(t, &tr.Metadata, exceptions, "meta", 1, false, localhostIP, time.Now())
	})

	t.Run("overwrite-metadata", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var inputTr transaction
		var tr model.Transaction
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, false, time.Now())
		inputTr.Context.Request.Headers.Val.Add("user-agent", "first")
		inputTr.Context.Request.Headers.Val.Add("user-agent", "second")
		inputTr.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), true, &tr)

		// user-agent should be set to context request header values
		assert.Equal(t, "first, second", tr.Metadata.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		assert.Equal(t, localhostIP, tr.Metadata.Client.IP, tr.Metadata.Client.IP.String())
		// metadata labels and transaction labels should not be merged
		assert.Equal(t, common.MapStr{"meta": "meta"}, tr.Metadata.Labels)
		assert.Equal(t, &model.Labels{"overwritten": "overwritten"}, tr.Labels)
		// service values should be set
		modeldecodertest.AssertStructValues(t, &tr.Metadata.Service, exceptions, "overwritten", 100, true, localhostIP, time.Now())
		// user values should be set
		modeldecodertest.AssertStructValues(t, &tr.Metadata.User, exceptions, "overwritten", 100, true, localhostIP, time.Now())
	})

	t.Run("client-ip-header", func(t *testing.T) {
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.Request.Headers.Set(http.Header{})
		inputTr.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		inputTr.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&inputTr, &model.Metadata{}, time.Now(), false, &tr)
		assert.Equal(t, gatewayIP, tr.Metadata.Client.IP, tr.Metadata.Client.IP.String())
	})

	t.Run("client-ip-socket", func(t *testing.T) {
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&inputTr, &model.Metadata{}, time.Now(), false, &tr)
		assert.Equal(t, randomIP, tr.Metadata.Client.IP, tr.Metadata.Client.IP.String())
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), false, &tr)
		assert.Equal(t, "test@user.com", tr.Metadata.User.Email)
		assert.Zero(t, tr.Metadata.User.ID)
		assert.Zero(t, tr.Metadata.User.Name)
	})

	t.Run("other-transaction-values", func(t *testing.T) {
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

		var inputTr transaction
		var tr model.Transaction
		eventTime, reqTime := time.Now(), time.Now().Add(time.Second)
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, true, eventTime)
		mapToTransactionModel(&inputTr, initializedMeta(), reqTime, true, &tr)
		modeldecodertest.AssertStructValues(t, &tr, exceptions, "overwritten", 5000, true, localhostIP, eventTime)

		// set requestTime if eventTime is zero
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, true, time.Time{})
		mapToTransactionModel(&inputTr, initializedMeta(), reqTime, true, &tr)
		modeldecodertest.AssertStructValues(t, &tr, exceptions, "overwritten", 5000, true, localhostIP, reqTime)

	})

	t.Run("page.URL", func(t *testing.T) {
		var inputTr transaction
		inputTr.Context.Page.URL.Set("https://my.site.test:9201")
		var tr model.Transaction
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), false, &tr)
		assert.Equal(t, "https://my.site.test:9201", *tr.Page.URL.Full)
		assert.Equal(t, 9201, *tr.Page.URL.Port)
		assert.Equal(t, "https", *tr.Page.URL.Scheme)
	})

}

func TestTransactionValidationRules(t *testing.T) {
	testTransaction := func(t *testing.T, key string, tc testcase) {
		var event transaction
		r := reader(t, "transactions")
		modeldecodertest.DecodeDataWithReplacement(t, r, "transaction", key, tc.data, &event)

		// run validation and checks
		err := event.validate()
		if tc.errorKey == "" {
			assert.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorKey)
		}
	}

	t.Run("context", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "custom", data: `{"custom":{"k1":{"v1":123,"v2":"value"},"k2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
			{name: "custom-key-dot", errorKey: "patternKeys", data: `{"custom":{"k1.":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-asterisk", errorKey: "patternKeys", data: `{"custom":{"k1*":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-quote", errorKey: "patternKeys", data: `{"custom":{"k1\"":{"v1":123,"v2":"value"}}}`},
			{name: "tags", data: `{"tags":{"k1":"v1.s*\"","k2":34,"k3":23.56,"k4":true}}`},
			{name: "tags-key-dot", errorKey: "patternKeys", data: `{"tags":{"k1.":"v1"}}`},
			{name: "tags-key-asterisk", errorKey: "patternKeys", data: `{"tags":{"k1*":"v1"}}`},
			{name: "tags-key-quote", errorKey: "patternKeys", data: `{"tags":{"k1\"":"v1"}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":"abc"}}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"tags":{"k1":{"v1":[1,2,3]}}}`},
			{name: "tags-maxVal", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "tags-maxVal-exceeded", errorKey: "maxVals", data: `{"tags":{"k1":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	// this tests an arbitrary field to ensure the max rule works as expected
	t.Run("max", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "context-message-queue-name", data: `{"message":{"queue":{"name":"` + modeldecodertest.BuildString(1024) + `"}}}`},
			{name: "context-message-queue-name", errorKey: "max", data: `{"message":{"queue":{"name":"` + modeldecodertest.BuildString(1025) + `"}}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("request", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "request-body-string", data: `"body":"value"`},
			{name: "request-body-object", data: `"body":{"a":"b"}`},
			{name: "request-body-array", errorKey: "transaction.context.request.body", data: `"body":[1,2]`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"request":{"method":"get",` + tc.data + `}}`
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "service-name-az", data: `{"service":{"name":"abcdefghijklmnopqrstuvwxyz"}}`},
			{name: "service-name-AZ", data: `{"service":{"name":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"}}`},
			{name: "service-name-09 _-", data: `{"service":{"name":"0123456789 -_"}}`},
			{name: "service-name-invalid", errorKey: "regexpAlphaNumericExt", data: `{"service":{"name":"⌘"}}`},
			{name: "service-name-max", data: `{"service":{"name":"` + modeldecodertest.BuildStringWith(1024, '-') + `"}}`},
			{name: "service-name-max-exceeded", errorKey: "max", data: `{"service":{"name":"` + modeldecodertest.BuildStringWith(1025, '-') + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("duration", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "duration", data: `0.0`},
			{name: "duration", errorKey: "min", data: `-0.09`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "duration", tc)
			})
		}
	})

	t.Run("marks", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "marks", data: `{"k1":{"v1":12.3}}`},
			{name: "marks-dot", errorKey: "patternKeys", data: `{"k.1":{"v1":12.3}}`},
			{name: "marks-events-dot", errorKey: "patternKeys", data: `{"k1":{"v.1":12.3}}`},
			{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k*1":{"v1":12.3}}`},
			{name: "marks-events-asterisk", errorKey: "patternKeys", data: `{"k1":{"v*1":12.3}}`},
			{name: "marks-quote", errorKey: "patternKeys", data: `{"k\"1":{"v1":12.3}}`},
			{name: "marks-events-quote", errorKey: "patternKeys", data: `{"k1":{"v\"1":12.3}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "marks", tc)
			})
		}
	})

	t.Run("outcome", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "outcome-success", data: `"success"`},
			{name: "outcome-failure", data: `"failure"`},
			{name: "outcome-unknown", data: `"unknown"`},
			{name: "outcome-invalid", errorKey: "enum", data: `"anything"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "outcome", tc)
			})
		}
	})

	t.Run("url", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "port-string", data: `"port":"8200"`},
			{name: "port-int", data: `"port":8200`},
			{name: "port-invalid-type", errorKey: "types", data: `"port":[8200,8201]`},
			{name: "port-invalid-type", errorKey: "types", data: `"port":{"val":8200}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tc.data = `{"request":{"method":"get","url":{` + tc.data + `}}}`
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("user", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "id-string", data: `{"user":{"id":"user123"}}`},
			{name: "id-int", data: `{"user":{"id":44}}`},
			{name: "id-float", errorKey: "types", data: `{"user":{"id":45.6}}`},
			{name: "id-bool", errorKey: "types", data: `{"user":{"id":true}}`},
			{name: "id-string-max-len", data: `{"user":{"id":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "id-string-max-len-exceeded", errorKey: "max", data: `{"user":{"id":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "context", tc)
			})
		}
	})

	t.Run("required", func(t *testing.T) {
		// setup: create full metadata struct with arbitrary values set
		var event transaction
		modeldecodertest.InitStructValues(&event)
		// test vanilla struct is valid
		require.NoError(t, event.validate())

		// iterate through struct, remove every key one by one
		// and test that validation behaves as expected
		requiredKeys := map[string]interface{}{
			"duration":               nil,
			"id":                     nil,
			"span_count":             nil,
			"span_count.started":     nil,
			"trace_id":               nil,
			"type":                   nil,
			"context.request.method": nil,
		}
		modeldecodertest.SetZeroStructValue(&event, func(key string) {
			err := event.validate()
			if _, ok := requiredKeys[key]; ok {
				require.Error(t, err, key)
				assert.Contains(t, err.Error(), key)
			} else {
				assert.NoError(t, err, key)
			}
		})
	})
}
