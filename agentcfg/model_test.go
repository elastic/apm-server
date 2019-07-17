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
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestNewDoc(t *testing.T) {
	t.Run("InvalidInput", func(t *testing.T) {
		d, err := NewDoc([]byte("some string"))
		assert.Error(t, err)
		assert.Empty(t, d)
	})

	t.Run("EmptyInput", func(t *testing.T) {
		d, err := NewDoc([]byte{})
		require.NoError(t, err)
		expectedDoc := Doc{
			Settings: map[string]string{},
			ID:       fmt.Sprintf("%x", md5.Sum([]byte{}))}
		assert.Equal(t, &expectedDoc, d)
	})

	t.Run("ValidInput", func(t *testing.T) {
		inp := []byte(`{"_id": "1234", 
"_source": {"settings":{"sample_rate":0.5,"name":"testconfig","sampling":true,
"b":["b", "a"],
"nested":{"ab":"val","ac":[3,1,2],"aa":{"c":45.6}}}}}`)

		settings := Settings{
			"b":           "b,a",
			"sample_rate": "0.5",
			"name":        "testconfig",
			"sampling":    "true",
			"nested.ab":   "val",
			"nested.ac":   "3,1,2",
			"nested.aa.c": "45.6",
		}

		var b []byte
		b = append(b, []byte("b_b,a")...)
		b = append(b, []byte("name_testconfig")...)
		b = append(b, []byte("nested.aa.c_45.6")...)
		b = append(b, []byte("nested.ab_val")...)
		b = append(b, []byte("nested.ac_3,1,2")...)
		b = append(b, []byte("sample_rate_0.5")...)
		b = append(b, []byte("sampling_true")...)
		id := fmt.Sprintf("%x", md5.Sum(b))

		d, err := NewDoc(inp)
		require.NoError(t, err)
		assert.Equal(t, &Doc{Settings: settings, ID: id}, d)
	})
}
