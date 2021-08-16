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

package main

type field struct {
	Name        string `yaml:"name,omitempty"`
	Key         string `yaml:"key,omitempty"`
	Title       string `yaml:"title,omitempty"`
	Level       string `yaml:"level,omitempty"`
	Required    *bool  `yaml:"required,omitempty"`
	Type        string `yaml:"type,omitempty"`
	Format      string `yaml:"format,omitempty"`
	Description string `yaml:"description,omitempty"`

	// Dynamic controls whether a field is dynamically mapped.
	//
	// Note: we intentionally omit "dynamic: false", as we set
	// a default dynamic mapping value for each data stream and
	// opt *in* to dynamically mapping where needed.
	Dynamic bool `yaml:"dynamic,omitempty"`

	ObjectTypeParams interface{}            `yaml:"object_type_params,omitempty"`
	Release          string                 `yaml:"release,omitempty"`
	Alias            string                 `yaml:"alias,omitempty"`
	Path             string                 `yaml:"path,omitempty"`
	Footnote         string                 `yaml:"footnote,omitempty"`
	IgnoreAbove      *int                   `yaml:"ignore_above,omitempty"`
	MultiFields      []multiFieldDefinition `yaml:"multi_fields,omitempty"`
	Fields           []field                `yaml:"fields,omitempty"`
	MetricType       string                 `yaml:"metric_type,omitempty"`
	Unit             string                 `yaml:"unit,omitempty"`

	IsECS bool `yaml:"-"`
}

type multiFieldDefinition struct {
	Name         string `yaml:"name,omitempty"`
	Type         string `yaml:"type,omitempty"`
	Norms        *bool  `yaml:"norms,omitempty"`
	DefaultField *bool  `yaml:"default_field,omitempty"`
}
