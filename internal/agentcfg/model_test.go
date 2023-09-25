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

package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDoc(t *testing.T) {
	t.Run("InvalidInput", func(t *testing.T) {
		_, err := newResult([]byte("some string"), nil)
		assert.Error(t, err)
	})

	t.Run("EmptyInput", func(t *testing.T) {
		d, err := newResult([]byte{}, nil)
		require.NoError(t, err)
		assert.Equal(t, zeroResult(), d)
	})

	t.Run("ValidInput", func(t *testing.T) {
		inp := []byte(`{"_id": "1234", "_source": {"etag":"123", "settings":{"sample_rate":0.5}}}`)

		d, err := newResult(inp, nil)
		require.NoError(t, err)
		assert.Equal(t, Result{Source{Etag: "123", Settings: Settings{"sample_rate": "0.5"}}}, d)
	})
}

func TestQueryMarshaling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		out   string
	}{
		{name: "third_party",
			input: `{"service":{"name":"auth-service","environment":"production"},"mark_as_applied_by_agent":true}`,
			out:   `{"service":{"name":"auth-service","environment":"production"},"mark_as_applied_by_agent":true,"etag":""}`},
		{name: "elastic_apm",
			input: `{"service":{"name":"auth-service","environment":"production"},"etag":"1234"}`,
			out:   `{"service":{"name":"auth-service","environment":"production"},"etag":"1234"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var query Query
			require.NoError(t, json.Unmarshal([]byte(tc.input), &query))
			out, err := json.Marshal(query)
			require.NoError(t, err)
			assert.JSONEq(t, tc.out, string(out))
		})
	}
}
