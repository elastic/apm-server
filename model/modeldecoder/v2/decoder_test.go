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
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecodertest"
)

func TestResetModelOnRelease(t *testing.T) {
	inp := `{"metadata":{"service":{"name":"service-a"}}}`
	m := fetchMetadataRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(inp)).Decode(m))
	require.True(t, m.IsSet())
	releaseMetadataRoot(m)
	assert.False(t, m.IsSet())
}

func TestDecodeMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		decodeFn func(decoder.Decoder, *model.Metadata) error
	}{
		{name: "decodeMetadata", decodeFn: DecodeMetadata,
			input: `{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}`},
		{name: "decodeNestedMetadata", decodeFn: DecodeNestedMetadata,
			input: `{"metadata":{"service":{"name":"user-service","agent":{"name":"go","version":"1.0.0"}}}}`},
	} {
		t.Run("decode", func(t *testing.T) {
			var out model.Metadata
			dec := decoder.NewJSONDecoder(strings.NewReader(tc.input))
			require.NoError(t, tc.decodeFn(dec, &out))
			assert.Equal(t, model.Metadata{Service: model.Service{
				Name:  "user-service",
				Agent: model.Agent{Name: "go", Version: "1.0.0"}}}, out)

			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decode")
		})

		t.Run("validate", func(t *testing.T) {
			inp := `{}`
			var out model.Metadata
			err := tc.decodeFn(decoder.NewJSONDecoder(strings.NewReader(inp)), &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation")
		})
	}
}

func TestMapToMetadataModel(t *testing.T) {
	// setup:
	// create initialized modeldecoder and empty model metadata
	// map modeldecoder to model metadata and manually set
	// enhanced data that are never set by the modeldecoder
	var m metadata
	modeldecodertest.SetStructValues(&m, "init", 5000, false)
	var modelM model.Metadata
	ip := net.ParseIP("127.0.0.1")
	modelM.System.IP, modelM.Client.IP = ip, ip
	mapToMetadataModel(&m, &modelM)

	exceptions := func(key string) bool {
		if strings.HasPrefix(key, "UserAgent") {
			// these values are not set by modeldecoder
			return true
		}
		return false
	}

	// iterate through model and assert values are set
	assertStructValues(t, &modelM, exceptions, "init", 5000, false, ip)

	// overwrite model metadata with specified Values
	// then iterate through model and assert values are overwritten
	modeldecodertest.SetStructValues(&m, "overwritten", 12, true)
	mapToMetadataModel(&m, &modelM)
	assertStructValues(t, &modelM, exceptions, "overwritten", 12, true, ip)

	// map an empty modeldecoder metadata to the model
	// and assert values are unchanged
	modeldecodertest.SetZeroStructValues(&m)
	mapToMetadataModel(&m, &modelM)
	assertStructValues(t, &modelM, exceptions, "overwritten", 12, true, ip)
}

func TestDecodeTransaction(t *testing.T) {
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

func TestMapToTransactionModel(t *testing.T) {
	localhostIP := net.ParseIP("127.0.0.1")
	gatewayIP := net.ParseIP("192.168.0.1")
	randomIP := net.ParseIP("71.0.54.1")
	exceptions := func(key string) bool {
		return key == "RepresentativeCount"
	}

	initializedMeta := func() *model.Metadata {
		var inputMeta metadata
		var meta model.Metadata
		modeldecodertest.SetStructValues(&inputMeta, "meta", 1, false)
		mapToMetadataModel(&inputMeta, &meta)
		// initialize values that are not set by input
		meta.UserAgent = model.UserAgent{Name: "meta", Original: "meta"}
		meta.Client.IP = localhostIP
		meta.System.IP = localhostIP
		return &meta
	}

	t.Run("set metadata", func(t *testing.T) {
		// do not overwrite metadata with zero transaction values
		var inputTr transaction
		var tr model.Transaction
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), true, &tr)
		// iterate through metadata model and assert values are set
		assertStructValues(t, &tr.Metadata, exceptions, "meta", 1, false, localhostIP)
	})

	t.Run("overwrite metadata", func(t *testing.T) {
		// overwrite defined metadata with transaction metadata values
		var inputTr transaction
		var tr model.Transaction
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, false)
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
		assertStructValues(t, &tr.Metadata.Service, exceptions, "overwritten", 100, true, localhostIP)
		// user values should be set
		assertStructValues(t, &tr.Metadata.User, exceptions, "overwritten", 100, true, localhostIP)
	})

	t.Run("client-ip from header", func(t *testing.T) {
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.Request.Headers.Set(http.Header{})
		inputTr.Context.Request.Headers.Val.Add("x-real-ip", gatewayIP.String())
		inputTr.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&inputTr, &model.Metadata{}, time.Now(), false, &tr)
		assert.Equal(t, gatewayIP, tr.Metadata.Client.IP, tr.Metadata.Client.IP.String())
	})

	t.Run("client-ip from socket", func(t *testing.T) {
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.Request.Socket.RemoteAddress.Set(randomIP.String())
		mapToTransactionModel(&inputTr, &model.Metadata{}, time.Now(), false, &tr)
		assert.Equal(t, randomIP, tr.Metadata.Client.IP, tr.Metadata.Client.IP.String())
	})

	t.Run("overwrite user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var inputTr transaction
		var tr model.Transaction
		inputTr.Context.User.Email.Set("test@user.com")
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), false, &tr)
		assert.Equal(t, "test@user.com", tr.Metadata.User.Email)
		assert.Zero(t, tr.Metadata.User.ID)
		assert.Zero(t, tr.Metadata.User.Name)
	})

	t.Run("map non-metadata transaction values", func(t *testing.T) {
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
		modeldecodertest.SetStructValues(&inputTr, "overwritten", 5000, true)
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), true, &tr)
		assertStructValues(t, &tr, exceptions, "overwritten", 5000, true, localhostIP)

		// experimental values should only be set if allowed by configuration
		assert.Equal(t, "overwritten", tr.Experimental)
		tr = model.Transaction{}
		mapToTransactionModel(&inputTr, initializedMeta(), time.Now(), false, &tr)
		assert.Zero(t, tr.Experimental)

		// if timestamp is zero fall back to request time

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

func assertStructValues(t *testing.T, i interface{}, isException func(string) bool,
	vStr string, vInt int, vBool bool, vIP net.IP) {
	modeldecodertest.IterateStruct(i, func(f reflect.Value, key string) {
		if isException(key) {
			return
		}
		fVal := f.Interface()
		var newVal interface{}
		switch fVal.(type) {
		case map[string]interface{}:
			newVal = map[string]interface{}{vStr: vStr}
		case common.MapStr:
			newVal = common.MapStr{vStr: vStr}
		case *model.Labels:
			newVal = &model.Labels{vStr: vStr}
		case *model.Custom:
			newVal = &model.Custom{vStr: vStr}
		case model.TransactionMarks:
			newVal = model.TransactionMarks{vStr: model.TransactionMark{vStr: float64(vInt) + 0.5}}
		case []string:
			newVal = []string{vStr}
		case []int:
			newVal = []int{vInt, vInt}
		case string:
			newVal = vStr
		case *string:
			newVal = &vStr
		case int:
			newVal = vInt
		case *int:
			newVal = &vInt
		case float64:
			newVal = float64(vInt) + 0.5
		case *float64:
			val := float64(vInt) + 0.5
			newVal = &val
		case net.IP:
			newVal = vIP
		case bool:
			newVal = vBool
		case *bool:
			newVal = &vBool
		case http.Header:
			newVal = http.Header{vStr: []string{vStr, vStr}}
		default:
			// the populator recursively iterates over struct and structPtr
			// calling this function for all fields;
			// it is enough to only assert they are not zero here
			if f.Type().Kind() == reflect.Struct {
				assert.NotZero(t, f, key)
				return
			}
			if f.Type().Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Struct {
				assert.NotZero(t, f, key)
				return
			}
			panic(fmt.Sprintf("unhandled type %T for key %s", f.Type().Kind(), key))
		}
		assert.Equal(t, newVal, fVal, key)
	})
}
