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
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
)

// DecodeData decodes input from the io.Reader into the given output
// it skips events with a different type than the given eventType
// and decodes the first matching event type
func DecodeData(t *testing.T, r io.Reader, eventType string, out interface{}) {
	dec := newNDJSONStreamDecoder(r, 300*1024)
	var et string
	var err error
	for et != eventType {
		et, err = readEventType(dec)
		require.NoError(t, err)
	}
	// decode data
	require.NoError(t, dec.decode(&out))
}

// DecodeDataWithReplacement decodes input from the io.Reader and replaces data for the
// given key with the provided newData before decoding into the output
func DecodeDataWithReplacement(t *testing.T, r io.Reader, eventType string, newData string, out interface{}, keys ...string) {
	var data map[string]interface{}
	DecodeData(t, r, eventType, &data)
	// replace data for given key with newData
	d := data[eventType].(map[string]interface{})
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		if _, ok := d[key]; !ok {
			d[key] = map[string]interface{}{}
		}
		d = d[key].(map[string]interface{})
	}
	var keyData interface{}
	require.NoError(t, json.Unmarshal([]byte(newData), &keyData))
	d[keys[len(keys)-1]] = keyData

	// unmarshal data into  struct
	b, err := json.Marshal(data[eventType])
	require.NoError(t, err)
	require.NoError(t, decoder.NewJSONIteratorDecoder(bytes.NewReader(b)).Decode(out))
}

func readEventType(d *ndjsonStreamDecoder) (string, error) {
	body, err := d.readAhead()
	if err != nil && err != io.EOF {
		return "", err
	}
	body = bytes.TrimLeft(body, `{ "`)
	end := bytes.Index(body, []byte(`"`))
	if end == -1 {
		return "", errors.New("invalid input: " + string(body))
	}
	return string(body[0:end]), nil
}
