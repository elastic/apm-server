package package_tests

import (
	"testing"

	sm "github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	tests.TestPayloadAttributesInSchema(t,
		"sourcemap",
		tests.NewSet("sourcemap", "sourcemap.file", "sourcemap.names", "sourcemap.sources", "sourcemap.sourceRoot",
			"sourcemap.mappings", "sourcemap.sourcesContent", "sourcemap.version"),
		sm.Schema())
}

var (
	procSetup = tests.ProcessorSetup{
		Proc:            sm.NewProcessor(),
		FullPayloadPath: "../testdata/sourcemap/payload.json",
		TemplatePaths:   []string{"../_meta/fields.yml"},
	}
)

func TestAttributesPresenceInSourcemap(t *testing.T) {
	requiredKeys := tests.NewSet("service_name", "service_version",
		"bundle_filepath", "sourcemap")
	procSetup.AttrsPresence(t, requiredKeys, nil)
}

func TestKeywordLimitationOnSourcemapAttributes(t *testing.T) {
	mapping := map[string]string{
		"sourcemap.service.name":    "service_name",
		"sourcemap.service.version": "service_version",
		"sourcemap.bundle_filepath": "bundle_filepath",
	}
	procSetup.KeywordLimitation(t, tests.NewSet(), mapping)
}

func TestPayloadDataForSourcemap(t *testing.T) {
	type val = []interface{}
	payloadData := []tests.SchemaTestData{
		// add test data for testing
		// * specific edge cases
		// * multiple allowed dataypes
		// * regex pattern, time formats
		// * length restrictions, other than keyword length restrictions

		{Key: "sourcemap", Invalid: []tests.Invalid{
			{Msg: `error validating sourcemap`, Values: val{""}},
			{Msg: `sourcemap not in expected format`, Values: val{[]byte{}}}}},
		{Key: "service_name", Valid: val{tests.Str1024},
			Invalid: []tests.Invalid{
				{Msg: `service_name/minlength`, Values: val{""}},
				{Msg: `service_name/maxlength`, Values: val{tests.Str1025}},
				{Msg: `service_name/pattern`, Values: val{tests.Str1024Special}}}},
		{Key: "service_version", Valid: val{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `service_version/minlength`, Values: val{""}}}},
		{Key: "bundle_filepath", Valid: []interface{}{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `bundle_filepath/minlength`, Values: val{""}}}},
	}
	procSetup.DataValidation(t, payloadData)
}
