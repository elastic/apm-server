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

	"github.com/fatih/set"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

func TestFields(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, er.NewProcessor)

	notInEvent := set.New(
		"context.db.instance",
		"context.db.statement",
		"context.db.user",
		"context.db.type",
		"context.db",
		"context.user.user-agent",
		"context.user.ip",
		"context.system.ip",
		"listening",
		"error id icon",
		"view errors",
	)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, er.NewProcessor, notInEvent)
}
