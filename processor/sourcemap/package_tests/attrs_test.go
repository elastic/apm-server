package package_tests

import (
	"testing"

	sm "github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/sourcemap/generated/schema"
	"github.com/elastic/apm-server/tests"
)

var (
	procSetup = tests.ProcessorSetup{
		Proc:            sm.NewProcessor(),
		FullPayloadPath: "../testdata/sourcemap/payload.json",
		TemplatePaths:   []string{"../_meta/fields.yml"},
		Schema:          schema.PayloadSchema,
	}
)

func TestPayloadAttrsMatchFields(t *testing.T) {
	procSetup.PayloadAttrsMatchFields(t, tests.NewSet("sourcemap"), tests.NewSet())
}

func TestPayloadAttrsMatchJsonSchema(t *testing.T) {
	procSetup.PayloadAttrsMatchJsonSchema(t,
		tests.NewSet("sourcemap", "sourcemap.file", "sourcemap.names",
			"sourcemap.sources", "sourcemap.sourceRoot"), tests.NewSet())
}

func TestAttributesPresenceRequirementInSourcemap(t *testing.T) {
	required := tests.NewSet("service_name", "service_version", "bundle_filepath", "sourcemap")
	procSetup.AttrsPresence(t, required, nil)
	procSetup.AttrsNotNullable(t, required)
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
