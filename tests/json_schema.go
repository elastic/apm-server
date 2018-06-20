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
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
)

type Schema struct {
	Title                string
	Properties           map[string]*Schema
	AdditionalProperties map[string]interface{}
	PatternProperties    map[string]interface{}
	Items                *Schema
	MaxLength            int
}
type Mapping struct {
	from string
	to   string
}

func TestPayloadAttributesInSchema(t *testing.T, name string, undocumentedAttrs *Set, schema string) {
	payload, _ := loader.LoadValidData(name)
	jsonNames := NewSet()
	flattenJsonKeys(payload, "", jsonNames)
	jsonNamesDoc := Difference(jsonNames, undocumentedAttrs)

	schemaStruct, _ := schemaStruct(strings.NewReader(schema))
	schemaNames := NewSet()
	flattenSchemaNames(schemaStruct, "", addAllPropNames, schemaNames)

	missing := Difference(jsonNamesDoc, schemaNames)
	if missing.Len() > 0 {
		msg := fmt.Sprintf("Json payload fields missing in Schema %v", missing)
		assert.Fail(t, msg)
	}

	missing = Difference(schemaNames, jsonNames)
	if missing.Len() > 0 {
		msg := fmt.Sprintf("Json schema fields missing in Payload %v", missing)
		assert.Fail(t, msg)
	}
}

func TestJsonSchemaKeywordLimitation(t *testing.T, fieldPaths []string, schema string, exceptions *Set) {
	fieldNames, err := fetchFlattenedFieldNames(fieldPaths, addKeywordFields)
	assert.NoError(t, err)

	schemaStruct, _ := schemaStruct(strings.NewReader(schema))
	schemaNames := NewSet()
	flattenSchemaNames(schemaStruct, "", addLengthRestrictedPropNames, schemaNames)

	mapping := []Mapping{
		{"errors.context", "context"},
		{"transactions.context", "context"},
		{"errors.transaction.id", "transaction.id"},
		{"errors", "error"},
		{"transactions.spans", "span"},
		{"transactions", "transaction"},
		{"service", "context.service"},
		{"process", "context.process"},
		{"system", "context.system"},
	}

	mappedSchemaNames := NewSet()
	for _, k := range schemaNames.Array() {
		name := k.(string)
		for _, m := range mapping {
			if strings.HasPrefix(name, m.from) {
				k = strings.Replace(name, m.from, m.to, 1)
				break
			}
		}
		mappedSchemaNames.Add(k)
	}
	found := Union(mappedSchemaNames, exceptions)
	diff := SymmDifference(fieldNames, found)

	errMsg := fmt.Sprintf("Missing json schema length limit 1024 for: %v", diff)
	assert.Equal(t, 0, diff.Len(), errMsg)
}

func schemaStruct(reader io.Reader) (*Schema, error) {
	decoder := json.NewDecoder(reader)
	var schema Schema
	err := decoder.Decode(&schema)
	return &schema, err
}

func flattenSchemaNames(s *Schema, prefix string, addFn addProperty, flattened *Set) {
	if len(s.Properties) > 0 {
		for k, v := range s.Properties {
			flattenedKey := StrConcat(prefix, k, ".")
			if addFn(v) {
				flattened.Add(flattenedKey)
			}
			flattenSchemaNames(v, flattenedKey, addFn, flattened)
		}
	} else if s.Items != nil {
		flattenSchemaNames(s.Items, prefix, addFn, flattened)
	}
}

func flattenJsonKeys(data interface{}, prefix string, flattened *Set) {
	if d, ok := data.(map[string]interface{}); ok {
		for k, v := range d {
			key := StrConcat(prefix, k, ".")
			flattened.Add(key)
			flattenJsonKeys(v, key, flattened)
		}
	} else if d, ok := data.([]interface{}); ok {
		for _, v := range d {
			flattenJsonKeys(v, prefix, flattened)
		}
	}
}

type addProperty func(s *Schema) bool

func addAllPropNames(s *Schema) bool { return true }

func addLengthRestrictedPropNames(s *Schema) bool {
	if s.MaxLength == 1024 {
		return true
	} else if val, ok := s.AdditionalProperties["maxLength"]; ok {
		if valF, okF := val.(float64); okF && valF == 1024 {
			return true
		}
	} else if len(s.PatternProperties) > 0 {
		for _, v := range s.PatternProperties {
			val, ok := v.(map[string]interface{})["maxLength"]
			if ok && val.(float64) == 1024 {
				continue
			} else {
				return false
			}
		}
		return true
	}
	return false
}

type SchemaTestData struct {
	File  string
	Error string
}

func TestDataAgainstProcessor(t *testing.T, p processor.Processor, testData []SchemaTestData) {
	for _, d := range testData {
		data, err := loader.LoadData(d.File)
		assert.Nil(t, err)
		err = p.Validate(data)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), d.Error)
		}
	}
}
