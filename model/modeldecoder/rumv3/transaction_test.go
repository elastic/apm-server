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

func TestTransactionSetResetIsSet(t *testing.T) {
	var tRoot transactionRoot
	modeldecodertest.DecodeData(t, testdataReader(t, "rum_events"), "x", &tRoot)
	require.True(t, tRoot.IsSet())
	// call Reset and ensure initial state, except for array capacity
	tRoot.Reset()
	assert.False(t, tRoot.IsSet())
}

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
		input := modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: true}}
		str := `{"x":{"d":100,"id":"100","tid":"1","t":"request","yc":{"sd":2},"exper":"test"}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))
		var out model.Transaction
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Equal(t, "request", out.Type)
		assert.Equal(t, "test", out.Experimental)
		// fall back to request time
		assert.Equal(t, now, out.Timestamp)

		// experimental should only be set if allowed by configuration
		input = modeldecoder.Input{Metadata: model.Metadata{}, RequestTime: now, Config: modeldecoder.Config{Experimental: false}}
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		out = model.Transaction{}
		require.NoError(t, DecodeNestedTransaction(dec, &input, &out))
		assert.Nil(t, out.Experimental)

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

	initializedMeta := func() *model.Metadata {
		var inputMeta metadata
		var meta model.Metadata
		modeldecodertest.SetStructValues(&inputMeta, "meta", 1, false, time.Now())
		mapToMetadataModel(&inputMeta, &meta)
		// initialize values that are not set by input
		meta.UserAgent = model.UserAgent{Name: "meta", Original: "meta"}
		meta.Client.IP = localhostIP
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
			// Page.URL parts are derived from url (separately tested)
			// exclude attributes that are not set for RUM
			if strings.HasPrefix(key, "Metadata") || strings.HasPrefix(key, "Page.URL") ||
				key == "HTTP.Request.Body" || key == "HTTP.Request.Cookies" || key == "HTTP.Request.Socket" ||
				key == "HTTP.Response.HeadersSent" || key == "HTTP.Response.Finished" ||
				key == "Experimental" || key == "RepresentativeCount" || key == "Message" ||
				key == "URL" {
				return true
			}
			return false
		}

		var inputTr transaction
		var tr model.Transaction
		eventTime, reqTime := time.Now(), time.Now().Add(time.Second)
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, true, eventTime)
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
		r := testdataReader(t, "rum_events")
		modeldecodertest.DecodeDataWithReplacement(t, r, "x", key, tc.data, &event)
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
			{name: "custom", data: `{"cu":{"k1":{"v1":123,"v2":"value"},"k2":34,"k3":[{"a.1":1,"b*\"":2}]}}`},
			{name: "custom-key-dot", errorKey: "patternKeys", data: `{"cu":{"k1.":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-asterisk", errorKey: "patternKeys", data: `{"cu":{"k1*":{"v1":123,"v2":"value"}}}`},
			{name: "custom-key-quote", errorKey: "patternKeys", data: `{"cu":{"k1\"":{"v1":123,"v2":"value"}}}`},
			{name: "tags", data: `{"g":{"k1":"v1.s*\"","k2":34,"k3":23.56,"k4":true}}`},
			{name: "tags-key-dot", errorKey: "patternKeys", data: `{"g":{"k1.":"v1"}}`},
			{name: "tags-key-asterisk", errorKey: "patternKeys", data: `{"g":{"k1*":"v1"}}`},
			{name: "tags-key-quote", errorKey: "patternKeys", data: `{"g":{"k1\"":"v1"}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"g":{"k1":{"v1":"abc"}}}`},
			{name: "tags-invalid-type", errorKey: "typesVals", data: `{"g":{"k1":{"v1":[1,2,3]}}}`},
			{name: "tags-maxVal", data: `{"g":{"k1":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "tags-maxVal-exceeded", errorKey: "maxVals", data: `{"g":{"k1":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "c", tc)
			})
		}
	})

	// this tests an arbitrary field to ensure the max rule works as expected
	t.Run("max", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "trace-id-max", data: `"` + modeldecodertest.BuildString(1024) + `"`},
			{name: "trace-id-max-exceeded", errorKey: "max", data: `"` + modeldecodertest.BuildString(1025) + `"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "tid", tc)
			})
		}
	})

	t.Run("service", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "service-name-az", data: `{"se":{"n":"abcdefghijklmnopqrstuvwxyz"}}`},
			{name: "service-name-AZ", data: `{"se":{"n":"ABCDEFGHIJKLMNOPQRSTUVWXYZ"}}`},
			{name: "service-name-09 _-", data: `{"se":{"n":"0123456789 -_"}}`},
			{name: "service-name-invalid", errorKey: "regexpAlphaNumericExt", data: `{"se":{"n":"⌘"}}`},
			{name: "service-name-max", data: `{"e":{"n":"` + modeldecodertest.BuildStringWith(1024, '-') + `"}}`},
			{name: "service-name-max-exceeded", errorKey: "max", data: `{"se":{"n":"` + modeldecodertest.BuildStringWith(1025, '-') + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "c", tc)
			})
		}
	})

	t.Run("duration", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "duration", data: `0.0`},
			{name: "duration", errorKey: "min", data: `-0.09`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "d", tc)
			})
		}
	})

	t.Run("marks", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "marks", data: `{"k1":{"v1":12.3}}`},
			{name: "marks-dot", errorKey: "patternKeys", data: `{"k.1":{"v1":12.3}}`},
			{name: "marks-event-dot", errorKey: "patternKeys", data: `{"k1":{"v.1":12.3}}`},
			{name: "marks-asterisk", errorKey: "patternKeys", data: `{"k*1":{"v1":12.3}}`},
			{name: "marks-event-asterisk", errorKey: "patternKeys", data: `{"k1":{"v*1":12.3}}`},
			{name: "marks-quote", errorKey: "patternKeys", data: `{"k\"1":{"v1":12.3}}`},
			{name: "marks-event-quote", errorKey: "patternKeys", data: `{"k1":{"v\"1":12.3}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "k", tc)
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
				testTransaction(t, "o", tc)
			})
		}
	})

	t.Run("user", func(t *testing.T) {
		for _, tc := range []testcase{
			{name: "id-string", data: `{"u":{"id":"user123"}}`},
			{name: "id-int", data: `{"u":{"id":44}}`},
			{name: "id-float", errorKey: "types", data: `{"u":{"id":45.6}}`},
			{name: "id-bool", errorKey: "types", data: `{"u":{"id":true}}`},
			{name: "id-string-max-len", data: `{"u":{"id":"` + modeldecodertest.BuildString(1024) + `"}}`},
			{name: "id-string-max-len-exceeded", errorKey: "max", data: `{"u":{"id":"` + modeldecodertest.BuildString(1025) + `"}}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testTransaction(t, "c", tc)
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
		requiredKeys := map[string]interface{}{"d": nil,
			"id":     nil,
			"yc":     nil,
			"yc.sd":  nil,
			"tid":    nil,
			"t":      nil,
			"c.q.mt": nil,
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
