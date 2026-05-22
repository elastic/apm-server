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

package asserts

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// FieldHasType asserts that a field in a field_caps response is mapped
// exclusively as the expected type.
func FieldHasType(t *testing.T, fields map[string]map[string]types.FieldCapability, fieldName, expectedType string) {
	t.Helper()
	caps, ok := fields[fieldName]
	if !assert.True(t, ok, "field %q missing from field_caps response", fieldName) {
		return
	}
	typeNames := make([]string, 0, len(caps))
	for typeName := range caps {
		typeNames = append(typeNames, typeName)
	}
	assert.Len(t, typeNames, 1, "field %q has multiple types: %v", fieldName, typeNames)
	_, hasExpected := caps[expectedType]
	assert.True(t, hasExpected, "field %q expected type %q, got types: %v", fieldName, expectedType, typeNames)
}
