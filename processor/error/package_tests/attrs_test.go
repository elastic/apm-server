// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func TestPayloadAttrsMatchFields(t *testing.T) {
	procSetup().PayloadAttrsMatchFields(t,
		payloadAttrsNotInFields(nil),
		fieldsNotInPayloadAttrs(tests.NewSet("parent", "parent.id", "trace", "trace.id")))
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
