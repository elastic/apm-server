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

package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSchemaInvalidResource(t *testing.T) {
	invalid := `{`
	assert.Panics(t, func() { CreateSchema(invalid, "myschema") })
}

func TestCreateSchemaInvalidSchema(t *testing.T) {
	assert.Panics(t, func() { CreateSchema(invalidSchema, "myschema") })
}

func TestCreateSchemaOK(t *testing.T) {
	schema := CreateSchema(validSchema, "myschema")
	assert.NotNil(t, schema)
}

func TestValidateFails(t *testing.T) {
	data := map[string]interface{}{"age": 12}
	schema := CreateSchema(validSchema, "myschema")
	err := Validate(data, schema)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing properties: \"name\""))
}

func TestValidateOK(t *testing.T) {
	data := map[string]interface{}{"name": "john"}
	schema := CreateSchema(validSchema, "myschema")
	err := Validate(data, schema)
	assert.Nil(t, err)
}

var invalidSchema = `{
  "id": "person",
  "type": "object",
  "properties": {
    "name":{
      "type": "unknown"
    }
  }
}`

var validSchema = `{
  "id": "person",
  "type": "object",
  "properties": {
    "name":{
      "type": "string"
    },
    "age":{
      "description": "some age",
      "type": "number"
    }
  },
	"required": ["name"]
}`
