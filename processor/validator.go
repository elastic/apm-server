package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"io"

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

func Validate(reader io.Reader, schema *jsonschema.Schema, data interface{}) error {
	var buf bytes.Buffer
	dataReader := io.TeeReader(reader, &buf)
	err := schema.Validate(dataReader)
	if err != nil {
		return fmt.Errorf("Problem validating JSON document against schema: %v", err)
	}

	return json.Unmarshal(buf.Bytes(), data)
}
