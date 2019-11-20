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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
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
