package processor

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema"
)

func CreateSchema(schemaData string, url string) *jsonschema.Schema {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, strings.NewReader(schemaData)); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		panic(err)
	}
	return schema
}

func Validate(data []byte, schema *jsonschema.Schema) error {
	if err := schema.Validate(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("Problem validating JSON document against schema: %v", err)
	}
	return nil
}
