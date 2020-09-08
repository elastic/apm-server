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

package modeldecodertest

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
)

func DecodeTestData(t *testing.T, r io.Reader, eventType string, out interface{}) {
	dec := decoder.NewJSONDecoder(r)
	// skip first line (metadata) for all events but metadata
	if eventType != "metadata" && eventType != "m" {
		var data interface{}
		require.NoError(t, dec.Decode(&data))
	}
	// decode data
	require.NoError(t, dec.Decode(&out))
}

func ReplaceTestData(t *testing.T, r io.Reader, eventType string, key string, newData string, out interface{}) {
	var data map[string]interface{}
	DecodeTestData(t, r, eventType, &data)
	// replace data for given key with newData
	eventData := data[eventType].(map[string]interface{})
	var keyData interface{}
	require.NoError(t, json.Unmarshal([]byte(newData), &keyData))
	eventData[key] = keyData

	// unmarshal data into  struct
	b, err := json.Marshal(eventData)
	require.NoError(t, err)
	require.NoError(t, decoder.NewJSONDecoder(bytes.NewReader(b)).Decode(out))
}
