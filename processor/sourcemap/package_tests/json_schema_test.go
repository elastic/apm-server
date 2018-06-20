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

	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	tests.TestPayloadAttributesInSchema(t,
		"sourcemap",
		tests.NewSet("sourcemap", "sourcemap.file", "sourcemap.names", "sourcemap.sources", "sourcemap.sourceRoot",
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
