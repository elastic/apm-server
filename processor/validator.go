package processor

import (
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

func Validate(raw interface{}, schema *jsonschema.Schema) error {
	if err := schema.ValidateInterface(raw); err != nil {
		return fmt.Errorf("Problem validating JSON document against schema: %v", err)
	}
	return nil
}
