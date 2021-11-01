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
	"fmt"

	libtemplate "github.com/elastic/beats/v7/libbeat/template"
)

//Template returns a template configuration with appropriate ILM settings
func Template(ilmEnabled, overwrite bool, name string, policy string) libtemplate.TemplateConfig {
	template := libtemplate.TemplateConfig{
		Enabled:   true,
		Name:      name,
		Pattern:   fmt.Sprintf("%s*", name),
		Overwrite: overwrite,
		Order:     2,
		Type:      libtemplate.IndexTemplateLegacy,
	}
	if ilmEnabled {
		template.Settings = libtemplate.TemplateSettings{
			Index: map[string]interface{}{
				"lifecycle.name":           policy,
				"lifecycle.rollover_alias": name,
			},
		}
	}
	return template
}
