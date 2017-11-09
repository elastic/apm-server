package tests

import (
	"encoding/json"
	"fmt"

	"testing"

	"io/ioutil"

	"path/filepath"
	"runtime"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"
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

// not exhaustive list, just the words currently used in our spec
var jsonSchemaReservedWords = []string{"$schema", "$id", "type", "description", "anyOf", "required", "title", "items",
	"minItems", "allOf", "properties", "maxLength", "pattern", "enum", "default", "format", "additionalProperties",
	"regexProperties", "dependencies"}

func TestPayloadAttributesInSchema(t *testing.T, name string, undocumented *set.Set, specDir string) {

	schema, specDir := loadSchema(specDir, "")
	schemaNames := set.New()
	flattenJsonKeys(schemaNames, schema, specDir, "")

	var payload map[string]interface{}
	UnmarshalValidData(name, &payload)
	jsonNames := set.New()
	flattenJsonKeys(jsonNames, payload, "", "")
	jsonNames = set.Difference(jsonNames, undocumented).(*set.Set)

	missing := set.Difference(jsonNames, schemaNames).(*set.Set)

	if missing.Size() > 0 {
		msg := fmt.Sprintf("Json Transaction Payload fields missing in Schema %v", missing)
		assert.Fail(t, msg)
	}

	missing = set.Difference(schemaNames, jsonNames).(*set.Set)
	if missing.Size() > 0 {
		msg := fmt.Sprintf("Json Transaction schema fields missing in Payload %v", missing)
		assert.Fail(t, msg)
	}
}

// loads jsonSchema file (.json) given a current directory, and returns its contents and directory
// current directories need to be propagate across calls to loadSchema because $ref paths are relative
func loadSchema(spec string, currentDir string) (map[string]interface{}, string) {
	if currentDir == "" {
		_, current, _, _ := runtime.Caller(0)
		currentDir = filepath.Join(filepath.Dir(current), "../docs/spec/")
	}
	specFile := filepath.Join(currentDir, spec)
	schemaData, _ := ioutil.ReadFile(specFile)
	var schema map[string]interface{}
	json.Unmarshal(schemaData, &schema)
	return schema, filepath.Dir(specFile)
}

// walks a json collecting nested keys separated by dots, ie. {a:{b:c:{d:1}} ==> a.b.c.d
// accumulates the result in the first argument, which is mutated recursively
// it has some rudimentary knowledge of jsonSchema.
// jsonSchema keys are traversed, but not accumulated in the result
func flattenJsonKeys(flattened *set.Set, data interface{}, currentSpecDir string, keyPrefix string) {
	if d, ok := data.(map[string]interface{}); ok {
		for k, v := range d {
			key := StrConcat(keyPrefix, k, ".")
			if Contains(jsonSchemaReservedWords, k) {
				// if we find a jsonSchema key, we walk it down without "capturing" the key
				// eg: {properties: { app {properties: name: {...}}}} becomes just app.name
				flattenJsonKeys(flattened, v, currentSpecDir, keyPrefix)
			} else if k == "$ref" {
				// if we find a reference to another file, we load it and continue from there
				schema, newCurrentSpecDir := loadSchema(v.(string), currentSpecDir)
				flattenJsonKeys(flattened, schema, newCurrentSpecDir, keyPrefix)
			} else if k == "patternProperties" {
				// jsonSchema allows values as keys under patternProperties key
				// we need to stop here so to not confuse arbitrary regex values with actual keys
				continue
			} else {
				flattened.Add(key)
				flattenJsonKeys(flattened, v, currentSpecDir, key)
			}
		}
	} else if d, ok := data.([]interface{}); ok {
		for _, v := range d {
			flattenJsonKeys(flattened, v, currentSpecDir, keyPrefix)
		}
	}
}
