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

	"github.com/elastic/beats/libbeat/common"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

func TestConfig(t *testing.T) {

	t.Run("ValidInput", func(t *testing.T) {
		for name, tc := range map[string]struct {
			input string
			mode  libilm.Mode
		}{
			"True":  {input: "true", mode: libilm.ModeEnabled},
			"False": {input: "false", mode: libilm.ModeDisabled},
			"Auto":  {input: "auto", mode: libilm.ModeAuto},
		} {
			t.Run(name, func(t *testing.T) {
				inp, err := common.NewConfigFrom(map[string]string{"enabled": tc.input})
				require.NoError(t, err)

				var cfg Config
				require.NoError(t, inp.Unpack(&cfg))
				assert.Equal(t, tc.mode, cfg.Mode)
			})
		}
	})

	t.Run("Invalid Input", func(t *testing.T) {
		for _, input := range []string{"invalid", ""} {
			inp, err := common.NewConfigFrom(map[string]string{"enabled": input})
			require.NoError(t, err)

			var cfg Config
			assert.Error(t, inp.Unpack(&cfg))
		}
	})

}
