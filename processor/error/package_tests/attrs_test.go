package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func TestPayloadAttrsMatchFields(t *testing.T) {
	procSetup().PayloadAttrsMatchFields(t,
		payloadAttrsNotInFields(nil), fieldsNotInPayloadAttrs(nil))
}

func TestPayloadAttrsMatchJsonSchema(t *testing.T) {
	procSetup().PayloadAttrsMatchJsonSchema(t,
		payloadAttrsNotInJsonSchema(nil), tests.NewSet(nil))
}

func TestAttrsPresenceInError(t *testing.T) {
	procSetup().AttrsPresence(t, requiredKeys(nil), condRequiredKeys(nil))
}

func TestKeywordLimitationOnErrorAttributes(t *testing.T) {
	procSetup().KeywordLimitation(t, keywordExceptionKeys(nil), templateToSchemaMapping(nil))
}

func TestPayloadDataForError(t *testing.T) {
	//// add test data for testing
	//// * specific edge cases
	//// * multiple allowed dataypes
	//// * regex pattern, time formats
	//// * length restrictions, other than keyword length restrictions
	procSetup().DataValidation(t, schemaTestData(nil))
}
