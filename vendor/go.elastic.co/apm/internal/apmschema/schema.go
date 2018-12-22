package apmschema

import (
	"go/build"
	"log"
	"path"
	"path/filepath"
	"runtime"

	"github.com/santhosh-tekuri/jsonschema"
)

var (
	// Error is the compiled JSON Schema for an error.
	Error *jsonschema.Schema

	// Metadata is the compiled JSON Schema for metadata.
	Metadata *jsonschema.Schema

	// MetricSet is the compiled JSON Schema for a set of metrics.
	MetricSet *jsonschema.Schema

	// Span is the compiled JSON Schema for a span.
	Span *jsonschema.Schema

	// Transaction is the compiled JSON Schema for a transaction.
	Transaction *jsonschema.Schema
)

func init() {
	pkg, err := build.Default.Import("go.elastic.co/apm/internal/apmschema", "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft4
	schemaDir := path.Join(filepath.ToSlash(pkg.Dir), "jsonschema")
	if runtime.GOOS == "windows" {
		schemaDir = "/" + schemaDir
	}
	compile := func(filepath string, out **jsonschema.Schema) {
		schema, err := compiler.Compile("file://" + path.Join(schemaDir, filepath))
		if err != nil {
			log.Fatal(err)
		}
		*out = schema
	}
	compile("errors/v2_error.json", &Error)
	compile("metadata.json", &Metadata)
	compile("metricsets/v2_metricset.json", &MetricSet)
	compile("spans/v2_span.json", &Span)
	compile("transactions/v2_transaction.json", &Transaction)
}
