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

package ilm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
)

func TestMakeDefaultSupporter(t *testing.T) {
	info := beat.Info{Beat: "mockapm", Version: "9.9.9"}

	t.Run("missing index", func(t *testing.T) {
		cfg := Config{Policies: []Policy{{EventType: "abc", Policy: map[string]interface{}{}}}}
		indexNames := map[string]string{}
		s, err := MakeDefaultSupporter(nil, info, 0, cfg, indexNames)
		assert.Nil(t, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index name missing")
	})
	t.Run("invalid index name", func(t *testing.T) {
		cfg := Config{Policies: []Policy{{EventType: "error", Policy: map[string]interface{}{}}}}
		indexNames := map[string]string{"error": "%{[xyz.name]}-%{[observer.version]}-%{[beat.name]}-%{[beat.version]}"}
		s, err := MakeDefaultSupporter(nil, info, 0, cfg, indexNames)
		assert.Nil(t, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key not found")
	})
	t.Run("valid", func(t *testing.T) {
		cfg := Config{Policies: []Policy{
			{EventType: "error", Policy: map[string]interface{}{"a": "b"}, Name: "foo"},
			{EventType: "transaction", Policy: map[string]interface{}{"b": "c"}},
		}}
		indexNames := map[string]string{
			"error":       "%{[observer.name]}-%{[observer.version]}-%{[beat.name]}-%{[beat.version]}",
			"transaction": "apm-8.0.0-transaction",
		}
		s, err := MakeDefaultSupporter(nil, info, 0, cfg, indexNames)
		assert.Equal(t, 2, len(s))
		assert.Equal(t, "mockapm-9.9.9-mockapm-9.9.9", s[0].Alias().Name)
		assert.Equal(t, "foo", s[0].Policy().Name)
		require.NoError(t, err)
	})
}
