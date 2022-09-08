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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/decoder"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/model/modeldecoder"
	"github.com/elastic/apm-server/internal/model/modeldecoder/modeldecodertest"
	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestResetLogOnRelease(t *testing.T) {
	input := `{"log":{"message":"something happened"}}`
	root := fetchLogRoot()
	require.NoError(t, decoder.NewJSONDecoder(strings.NewReader(input)).Decode(root))
	require.True(t, root.IsSet())
	releaseLogRoot(root)
	assert.False(t, root.IsSet())
}

func TestDecodeNestedLog(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		now := time.Now()
		input := modeldecoder.Input{}
		str := `{"log":{"message":"something happened","timestamp":1662616971000000,"trace_id":"1","logger_name":"github.com/elastic/apm-server","severity":1}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))

		var batch model.Batch
		require.NoError(t, DecodeNestedLog(dec, &input, &batch))
		require.Len(t, batch, 1)
		assert.Equal(t, "something happened", batch[0].Message)
		assert.Equal(t, "1", batch[0].Trace.ID)
		assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", batch[0].Timestamp.String())

		input = modeldecoder.Input{Base: model.APMEvent{Timestamp: now}}
		str = `{"log":{"message":"something happened","trace_id":"1","logger_name":"github.com/elastic/apm-server","severity":1}}`
		dec = decoder.NewJSONDecoder(strings.NewReader(str))
		batch = model.Batch{}
		require.NoError(t, DecodeNestedLog(dec, &input, &batch))
		assert.Equal(t, now, batch[0].Timestamp)

		err := DecodeNestedLog(decoder.NewJSONDecoder(strings.NewReader(`malformed`)), &input, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})

	t.Run("validate", func(t *testing.T) {
		var batch model.Batch
		err := DecodeNestedLog(decoder.NewJSONDecoder(strings.NewReader(`{}`)), &modeldecoder.Input{}, &batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestDecodeMapToLogModel(t *testing.T) {
	t.Run("metadata-overwrite", func(t *testing.T) {
		// overwrite defined metadata with event metadata values
		var input log
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		otherVal := modeldecodertest.NonDefaultValues()
		modeldecodertest.SetStructValues(&input, otherVal)
		mapToLogModel(&input, &out)
		input.Reset()

		// ensure event Metadata are updated where expected
		otherVal = modeldecodertest.NonDefaultValues()
		userAgent := strings.Join(otherVal.HTTPHeader.Values("User-Agent"), ", ")
		assert.Equal(t, userAgent, out.UserAgent.Original)
		// do not overwrite client.ip if already set in metadata
		ip := modeldecodertest.DefaultValues().IP
		assert.Equal(t, ip, out.Client.IP, out.Client.IP.String())
		assert.Equal(t, model.Labels{
			"init0": {Global: true, Value: "init"}, "init1": {Global: true, Value: "init"}, "init2": {Global: true, Value: "init"},
			"overwritten0": {Value: "overwritten"}, "overwritten1": {Value: "overwritten"},
		}, out.Labels)
		exceptions := func(key string) bool { return false }
		modeldecodertest.AssertStructValues(t, &out.Service, exceptions, otherVal)
		modeldecodertest.AssertStructValues(t, &out.User, exceptions, otherVal)
	})

	t.Run("cloud.origin", func(t *testing.T) {
		var input log
		var out model.APMEvent
		origin := contextCloudOrigin{}
		origin.Account.ID.Set("accountID")
		origin.Provider.Set("aws")
		origin.Region.Set("us-east-1")
		origin.Service.Name.Set("serviceName")
		input.Context.Cloud.Origin = origin
		mapToLogModel(&input, &out)
		assert.Equal(t, "accountID", out.Cloud.Origin.AccountID)
		assert.Equal(t, "aws", out.Cloud.Origin.Provider)
		assert.Equal(t, "us-east-1", out.Cloud.Origin.Region)
		assert.Equal(t, "serviceName", out.Cloud.Origin.ServiceName)
	})

	t.Run("faas", func(t *testing.T) {
		var input log
		var out model.APMEvent
		input.FAAS.ID.Set("faasID")
		input.FAAS.Coldstart.Set(true)
		input.FAAS.Execution.Set("execution")
		input.FAAS.Trigger.Type.Set("http")
		input.FAAS.Trigger.RequestID.Set("abc123")
		input.FAAS.Name.Set("faasName")
		input.FAAS.Version.Set("1.0.0")
		mapToLogModel(&input, &out)
		assert.Equal(t, "faasID", out.FAAS.ID)
		assert.True(t, *out.FAAS.Coldstart)
		assert.Equal(t, "execution", out.FAAS.Execution)
		assert.Equal(t, "http", out.FAAS.TriggerType)
		assert.Equal(t, "abc123", out.FAAS.TriggerRequestID)
		assert.Equal(t, "faasName", out.FAAS.Name)
		assert.Equal(t, "1.0.0", out.FAAS.Version)
	})

	t.Run("overwrite-user", func(t *testing.T) {
		// user should be populated by metadata or event specific, but not merged
		var input log
		_, out := initializedInputMetadata(modeldecodertest.DefaultValues())
		input.Context.User.Email.Set("test@user.com")
		mapToLogModel(&input, &out)
		assert.Equal(t, "test@user.com", out.User.Email)
		assert.Zero(t, out.User.ID)
		assert.Zero(t, out.User.Name)
	})

	t.Run("http-headers", func(t *testing.T) {
		var input log
		input.Context.Request.Headers.Set(http.Header{"a": []string{"b"}, "c": []string{"d", "e"}})
		input.Context.Response.Headers.Set(http.Header{"f": []string{"g"}})
		var out model.APMEvent
		mapToLogModel(&input, &out)
		assert.Equal(t, mapstr.M{"a": []string{"b"}, "c": []string{"d", "e"}}, out.HTTP.Request.Headers)
		assert.Equal(t, mapstr.M{"f": []string{"g"}}, out.HTTP.Response.Headers)
	})

	t.Run("http-request-body", func(t *testing.T) {
		var input log
		input.Context.Request.Body.Set(map[string]interface{}{
			"a": json.Number("123.456"),
			"b": nil,
			"c": "d",
		})
		var out model.APMEvent
		mapToLogModel(&input, &out)
		assert.Equal(t, map[string]interface{}{"a": 123.456, "c": "d"}, out.HTTP.Request.Body)
	})

	t.Run("labels", func(t *testing.T) {
		var input log
		input.Context.Tags = mapstr.M{
			"a": "b",
			"c": float64(12315124131),
			"d": 12315124131.12315124131,
			"e": true,
		}
		var out model.APMEvent
		mapToLogModel(&input, &out)
		assert.Equal(t, model.Labels{
			"a": {Value: "b"},
			"e": {Value: "true"},
		}, out.Labels)
		assert.Equal(t, model.NumericLabels{
			"c": {Value: float64(12315124131)},
			"d": {Value: float64(12315124131.12315124131)},
		}, out.NumericLabels)
	})
}
