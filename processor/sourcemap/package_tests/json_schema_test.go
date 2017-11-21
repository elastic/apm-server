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
		"sourcemap/payload.json",
		set.New("sourcemap", "sourcemap.file", "sourcemap.names", "sourcemap.sources", "sourcemap.sourceRoot",
			"sourcemap.mappings", "sourcemap.sourcesContent", "sourcemap.version"),
		sourcemap.Schema())
}
