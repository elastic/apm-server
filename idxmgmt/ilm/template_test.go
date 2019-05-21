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

	libtemplate "github.com/elastic/beats/libbeat/template"
)

func TestTemplate_ILMEnabled(t *testing.T) {
	tmpl := Template(true, false, true, "mock-apm", "mock-apm-keep")
	expected := libtemplate.TemplateConfig{
		Enabled:   false,
		Name:      "mock-apm",
		Pattern:   "mock-apm*",
		Overwrite: true,
		Order:     2,
		Settings: libtemplate.TemplateSettings{
			Index: map[string]interface{}{
				"lifecycle.name":           "mock-apm-keep",
				"lifecycle.rollover_alias": "mock-apm",
			},
		},
	}
	assert.Equal(t, expected, tmpl)
}

func TestTemplate_ILMDisabled(t *testing.T) {
	tmpl := Template(false, true, false, "mock-apm", "mock-apm-keep")
	expected := libtemplate.TemplateConfig{
		Enabled:   true,
		Name:      "mock-apm",
		Pattern:   "mock-apm*",
		Overwrite: false,
		Order:     2,
	}
	assert.Equal(t, expected, tmpl)
}
