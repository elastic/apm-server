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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/decoder"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/model/modeldecoder"
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
		str := `{"log":{"message":"something happened","@timestamp":1662616971000000}}`
		dec := decoder.NewJSONDecoder(strings.NewReader(str))

		var batch model.Batch
		require.NoError(t, DecodeNestedLog(dec, &input, &batch))
		require.Len(t, batch, 1)
		assert.Equal(t, "something happened", batch[0].Message)
		assert.Equal(t, "2022-09-08 06:02:51 +0000 UTC", batch[0].Timestamp.String())

		input = modeldecoder.Input{Base: model.APMEvent{Timestamp: now}}
		str = `{"log":{"message":"something happened"}}`
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
}
