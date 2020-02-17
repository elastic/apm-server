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

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/elastic/apm-server/tests/loader"
)

func TestECSMapping(t *testing.T) {
	b, err := loader.LoadNDJSON("../_meta/ecs-migration.yml")
	require.NoError(t, err)

	type ECSFieldMigration struct {
		From, To   string
		Index      *bool
		Documented *bool
	}

	var ecsMigration []ECSFieldMigration
	err = yaml.Unmarshal(b, &ecsMigration)
	require.NoError(t, err)

	fieldNames, err := fetchFlattenedFieldNames([]string{"../_meta/fields.common.yml"})
	require.NoError(t, err)

	for _, field := range ecsMigration {
		if field.Index == nil || *field.Index || (field.Documented != nil && *field.Documented) {
			assert.True(t, fieldNames.Contains(field.To), "ECS field was expected in template: "+field.To)
			assert.False(t, fieldNames.Contains(field.From), "6.x field was not expected in template: "+field.From)
		} else {
			assert.False(t, fieldNames.Contains(field.To), "not indexed field not expected in template: "+field.To)
			assert.False(t, fieldNames.Contains(field.From), "not indexed field not expected in template "+field.From)
		}

	}

}
