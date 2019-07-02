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
	"github.com/elastic/beats/libbeat/common/fmtstr"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
)

const pattern = "000001"

// Config for ILM supporter
type Config struct {
	Mode       libilm.Mode               `config:"enabled"`
	PolicyName *fmtstr.EventFormatString `config:"policy_name"`
	AliasName  *fmtstr.EventFormatString `config:"alias_name"`
	Event      string                    `config:"event"`
}

// Enabled indicates whether or not ILM should be enabled
func (c *Config) Enabled() bool {
	return c.Mode != libilm.ModeDisabled
}

// ModeString stringifies enabled mode
// This is a workaround as strings should be generated in libbeat.
func ModeString(m libilm.Mode) string {
	switch m {
	case libilm.ModeAuto:
		return "auto"
	case libilm.ModeEnabled:
		return "true"
	default:
		return "false"
	}
}
