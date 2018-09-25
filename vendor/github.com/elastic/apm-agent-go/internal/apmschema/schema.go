package apmschema

import (
	"go/build"
	"log"
	"path"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema"
)

var (
	// Transactions is the compiled JSON Schema for a transactions payload.
	Transactions *jsonschema.Schema

	// Errors is the compiled JSON Schema for an errors payload.
	Errors *jsonschema.Schema

	// Metrics is the compiled JSON Schema for a metrics payload.
	Metrics *jsonschema.Schema
)

func init() {
	pkg, err := build.Default.Import("github.com/elastic/apm-agent-go/internal/apmschema", "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft4
	schemaDir := path.Join(filepath.ToSlash(pkg.Dir), "jsonschema")
	compile := func(filepath string, out **jsonschema.Schema) {
		schema, err := compiler.Compile("file://" + path.Join(schemaDir, filepath))
		if err != nil {
			log.Fatal(err)
		}
		*out = schema
	}
	compile("transactions/payload.json", &Transactions)
	compile("errors/payload.json", &Errors)
	compile("metrics/payload.json", &Metrics)
}
