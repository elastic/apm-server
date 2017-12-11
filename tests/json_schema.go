package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fatih/set"
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

func TestPayloadAttributesInSchema(t *testing.T, name string, undocumentedAttrs *set.Set, schema string) {
	payload, _ := loader.LoadValidData(name)
	jsonNames := set.New()
	flattenJsonKeys(payload, "", jsonNames)
	jsonNamesDoc := set.Difference(jsonNames, undocumentedAttrs).(*set.Set)

	schemaStruct, _ := schemaStruct(strings.NewReader(schema))
	schemaNames := set.New()
	flattenSchemaNames(schemaStruct, "", addAllPropNames, schemaNames)

	missing := set.Difference(jsonNamesDoc, schemaNames).(*set.Set)
	if missing.Size() > 0 {
		msg := fmt.Sprintf("Json payload fields missing in Schema %v", missing)
		assert.Fail(t, msg)
	}

	missing = set.Difference(schemaNames, jsonNames).(*set.Set)
	if missing.Size() > 0 {
		msg := fmt.Sprintf("Json schema fields missing in Payload %v", missing)
		assert.Fail(t, msg)
	}
}

func TestJsonSchemaKeywordLimitation(t *testing.T, fieldPaths []string, schema string, exceptions *set.Set) {
	fieldNames, err := fetchFlattenedFieldNames(fieldPaths, addKeywordFields)
	assert.NoError(t, err)

	schemaStruct, _ := schemaStruct(strings.NewReader(schema))
	schemaNames := set.New()
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

	mappedSchemaNames := set.New()
	for _, k := range schemaNames.List() {
		name := k.(string)
		for _, m := range mapping {
			if strings.HasPrefix(name, m.from) {
				k = strings.Replace(name, m.from, m.to, 1)
				break
			}
		}
		mappedSchemaNames.Add(k)
	}
	found := set.Union(mappedSchemaNames, exceptions).(*set.Set)
	diff := set.SymmetricDifference(fieldNames, found).(*set.Set)

	errMsg := fmt.Sprintf("Missing json schema length limit 1024 for: %v", diff)
	assert.Equal(t, 0, diff.Size(), errMsg)
}

func schemaStruct(reader io.Reader) (*Schema, error) {
	decoder := json.NewDecoder(reader)
	var schema Schema
	err := decoder.Decode(&schema)
	return &schema, err
}

func flattenSchemaNames(s *Schema, prefix string, addFn addProperty, flattened *set.Set) {
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

func flattenJsonKeys(data interface{}, prefix string, flattened *set.Set) {
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
		assert.Contains(t, err.Error(), d.Error)
	}
}
