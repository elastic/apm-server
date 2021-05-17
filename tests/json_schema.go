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

package tests

import (
	"encoding/json"
	"io"

	"github.com/elastic/beats/v7/libbeat/beat"
)

type TestProcessor interface {
	LoadPayload(string) (interface{}, error)
	Process([]byte) ([]beat.Event, error)
	Validate(interface{}) error
	Decode(interface{}) error
}

type ProcessorSetup struct {
	Proc TestProcessor
	// path to payload that should be a full and valid example
	FullPayloadPath string
	// path to ES template definitions
	TemplatePaths []string
	// path to json schema
	SchemaPath string
}

type Condition struct {
	// If requirements for a field apply in case of anothers key absence,
	// add the key.
	Absence []string
	// If requirements for a field apply in case of anothers key specific values,
	// add the key and its values.
	Existence map[string]interface{}
}

type obj = map[string]interface{}

type Schema struct {
	Title                string
	Properties           map[string]*Schema
	AdditionalProperties interface{} // bool or object
	PatternProperties    obj
	Items                *Schema
	AllOf                []*Schema
	OneOf                []*Schema
	AnyOf                []*Schema
	MaxLength            int
}

func ParseSchema(r io.Reader) (*Schema, error) {
	decoder := json.NewDecoder(r)
	var schema Schema
	err := decoder.Decode(&schema)
	return &schema, err
}

func FlattenSchemaNames(s *Schema, prefix string, filter func(*Schema) bool, flattened *Set) {
	if len(s.Properties) > 0 {
		for k, v := range s.Properties {
			key := strConcat(prefix, k, ".")
			if filter == nil || filter(v) {
				flattened.Add(key)
			}
			FlattenSchemaNames(v, key, filter, flattened)
		}
	}

	if s.Items != nil {
		FlattenSchemaNames(s.Items, prefix, filter, flattened)
	}

	for _, schemas := range [][]*Schema{s.AllOf, s.OneOf, s.AnyOf} {
		for _, e := range schemas {
			FlattenSchemaNames(e, prefix, filter, flattened)
		}
	}
	if filter(s) {
		flattened.Add(prefix)
	}
}

func flattenJsonKeys(data interface{}, prefix string, flattened *Set) {
	if d, ok := data.(obj); ok {
		for k, v := range d {
			key := strConcat(prefix, k, ".")
			flattened.Add(key)
			flattenJsonKeys(v, key, flattened)
		}
	} else if d, ok := data.([]interface{}); ok {
		for _, v := range d {
			flattenJsonKeys(v, prefix, flattened)
		}
	}
}
