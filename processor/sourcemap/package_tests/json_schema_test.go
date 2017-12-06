package package_tests

import (
	"testing"

	"github.com/fatih/set"

	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	tests.TestPayloadAttributesInSchema(t,
		"sourcemap",
		set.New("sourcemap", "sourcemap.file", "sourcemap.names", "sourcemap.sources", "sourcemap.sourceRoot",
			"sourcemap.mappings", "sourcemap.sourcesContent", "sourcemap.version"),
		sourcemap.Schema())
}

func TestSourcemapPayloadSchema(t *testing.T) {
	testData := []tests.SchemaTestData{
		{File: "data/invalid/sourcemap/no_service_version.json", Error: "missing properties: \"service_version\""},
		{File: "data/invalid/sourcemap/no_bundle_filepath.json", Error: "missing properties: \"bundle_filepath\""},
		{File: "data/invalid/sourcemap/not_allowed_empty_values.json", Error: "length must be >= 1, but got 0"},
		{File: "data/invalid/sourcemap/not_allowed_null_values.json", Error: "expected string, but got null"},
	}
	tests.TestDataAgainstProcessor(t, sourcemap.NewProcessor(), testData)
}
