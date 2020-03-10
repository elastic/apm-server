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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestFlattenCommonMapStr(t *testing.T) {
	emptyBlacklist := NewSet()
	blacklist := NewSet("a.bMap", "f")
	expectedAll := NewSet("a", "a.bStr", "a.bMap", "a.bMap.cMap", "a.bMap.cMap.d", "a.bMap.cStr", "a.bAnotherMap", "a.bAnotherMap.e", "f")
	expectedWoBlacklisted := NewSet("a", "a.bStr", "a.bMap.cStr", "a.bMap.cMap", "a.bMap.cMap.d", "a.bAnotherMap", "a.bAnotherMap.e")
	expectedAllPrefixed := NewSet("pre", "pre.a", "pre.a.bStr", "pre.a.bMap", "pre.a.bMap.cMap", "pre.a.bMap.cMap.d", "pre.a.bMap.cStr", "pre.a.bAnotherMap", "pre.a.bAnotherMap.e", "pre.f")
	expectedWithFilledInput := NewSet("prefilled", "a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e", "a.bMap.cMap.d", "a.bMap.cStr", "a.bMap.cMap")
	for idx, dataRow := range []struct {
		mapData   common.MapStr
		prefix    string
		blacklist *Set
		input     *Set
		retVal    *Set
	}{
		{common.MapStr{}, "whatever", emptyBlacklist, NewSet(), NewSet("whatever")},
		{common.MapStr{}, "", blacklist, NewSet(), NewSet()},
		{commonMapStr(), "", emptyBlacklist, NewSet(), expectedAll},
		{commonMapStr(), "", blacklist, NewSet(), expectedWoBlacklisted},
		{commonMapStr(), "pre", emptyBlacklist, NewSet(), expectedAllPrefixed},
		{commonMapStr(), "", blacklist, NewSet("prefilled"), expectedWithFilledInput},
	} {
		FlattenMapStr(dataRow.mapData, dataRow.prefix, dataRow.blacklist, dataRow.input)
		expected := dataRow.retVal
		diff := SymmDifference(dataRow.input, expected)

		errMsg := fmt.Sprintf("Failed for idx %v, diff: %v", idx, diff)
		assert.Equal(t, 0, diff.Len(), errMsg)
	}
}

func commonMapStr() common.MapStr {
	return common.MapStr{
		"a": common.MapStr{
			"bStr": "something",
			"bMap": common.MapStr{
				"cMap": common.MapStr{
					"d": "something",
				},
				"cStr": "",
			},
			"bAnotherMap": map[string]interface{}{
				"e": 0,
			},
		},
		"f": "",
	}
}

func TestLoadFields(t *testing.T) {
	_, err := loadFields("non-existing")
	assert.Error(t, err)
}

func TestFlattenFieldNames(t *testing.T) {
	fields, err := loadFields("./_meta/fields.yml")
	require.NoError(t, err)

	expectAll := NewSet("transaction", "transaction.id", "transaction.context", "exception", "exception.http",
		"exception.http.url", "exception.http.meta", "exception.stacktrace")
	expectDisabled := NewSet("transaction.context", "exception.stacktrace")
	expectEnabled := SymmDifference(expectAll, expectDisabled)

	allFields := NewSet()
	flattenFieldNames(fields, "", allFields, hasName)
	assert.Equal(t, expectAll, allFields)

	enabledFields := NewSet()
	flattenFieldNames(fields, "", enabledFields, hasName, isEnabled)
	assert.Equal(t, expectEnabled, enabledFields)

	disabledFields := NewSet()
	flattenFieldNames(fields, "", disabledFields, hasName, isDisabled)
	assert.Equal(t, expectDisabled, disabledFields)
}
