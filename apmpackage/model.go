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

package apmpackage

type field struct {
	Name        string                 `yaml:"name,omitempty"`
	Key         string                 `yaml:"key,omitempty"`
	Title       string                 `yaml:"title,omitempty"`
	Group       *int                   `yaml:"group,omitempty"`
	Level       string                 `yaml:"level,omitempty"`
	Required    *bool                  `yaml:"required,omitempty"`
	Type        string                 `yaml:"type,omitempty"`
	Format      string                 `yaml:"format,omitempty"`
	Description string                 `yaml:"description,omitempty"`
	Release     string                 `yaml:"release,omitempty"`
	Alias       string                 `yaml:"alias,omitempty"`
	Path        string                 `yaml:"path,omitempty"`
	Footnote    string                 `yaml:"footnote,omitempty"`
	IgnoreAbove *int                   `yaml:"ignore_above,omitempty"`
	MultiFields []multiFieldDefinition `yaml:"multi_fields,omitempty"`
	Fields      []field                `yaml:"fields,omitempty"`
	IsECS       bool                   `yaml:"-"`
	HasECS      bool                   `yaml:"-"`
	HasNonECS   bool                   `yaml:"-"`
}

func (f field) isNonECSLeaf() bool {
	return f.Type != "group" && !f.IsECS
}

type fields []field

func (f fields) Len() int           { return len(f) }
func (f fields) Less(i, j int) bool { return f[i].Name < f[j].Name }
func (f fields) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

type multiFieldDefinition struct {
	Name         string `yaml:"Name,omitempty"`
	Type         string `yaml:"type,omitempty"`
	Norms        *bool  `yaml:"norms,omitempty"`
	DefaultField *bool  `yaml:"default_field,omitempty"`
}

func copyFieldRoot(f field) field {
	return field{
		Name:        f.Name,
		Key:         f.Key,
		Title:       f.Title,
		Group:       f.Group,
		Level:       f.Level,
		Required:    f.Required,
		Type:        f.Title,
		Format:      f.Format,
		Description: f.Description,
		Release:     f.Release,
		Alias:       f.Alias,
		Path:        f.Path,
		Footnote:    f.Footnote,
		IgnoreAbove: f.IgnoreAbove,
		Fields:      nil,
		MultiFields: f.MultiFields,
		IsECS:       f.IsECS,
		HasECS:      f.HasECS,
		HasNonECS:   f.HasNonECS,
	}
}
