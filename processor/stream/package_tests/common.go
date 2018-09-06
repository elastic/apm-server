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
	"github.com/elastic/apm-server/tests"
	"github.com/santhosh-tekuri/jsonschema"
)

type TestSetup struct {
	InputDataPath string
	TemplatePaths []string
	Schema        *jsonschema.Schema
}

func metadataFields() *tests.Set {
	return tests.NewSet(
		// metadata fields
		"context.system",
		"context.service.language.name",
		"context.system.architecture",
		"context.service.language.version",
		"context.service.framework.name",
		"context.service.environment",
		"context.service.framework.version",
		"context.process.title",
		"context.service.name",
		"context.system.platform",
		"context.system.hostname",
		"context.service.version",
		"context.service.runtime.name",
		"context.service.agent.name",
		"context.service.agent.version",
		"context.service.runtime.version",
	)

}
