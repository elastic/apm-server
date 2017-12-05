package processor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
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

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (p Person) Transform() []beat.Event {
	return nil
}
