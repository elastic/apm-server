package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const mapDataIdx = 0
const prefixIdx = 1
const blacklistedIdx = 2
const inputIdx = 3
const retValIdx = 4

func TestFlattenCommonMapStr(t *testing.T) {
	emptyBlacklist := NewSet()
	blacklist := NewSet("a.bMap", "f")
	expectedAll := NewSet("a", "a.bStr", "a.bMap", "a.bMap.cMap", "a.bMap.cMap.d", "a.bMap.cStr", "a.bAnotherMap", "a.bAnotherMap.e", "f")
	expectedWoBlacklisted := NewSet("a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e")
	expectedAllPrefixed := NewSet("pre", "pre.a", "pre.a.bStr", "pre.a.bMap", "pre.a.bMap.cMap", "pre.a.bMap.cMap.d", "pre.a.bMap.cStr", "pre.a.bAnotherMap", "pre.a.bAnotherMap.e", "pre.f")
	expectedWithFilledInput := NewSet("prefilled", "a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e")
	data := [][]interface{}{
		[]interface{}{common.MapStr{}, "whatever", emptyBlacklist, NewSet(), NewSet("whatever")},
		[]interface{}{common.MapStr{}, "", blacklist, NewSet(), NewSet()},
		[]interface{}{commonMapStr(), "", emptyBlacklist, NewSet(), expectedAll},
		[]interface{}{commonMapStr(), "", blacklist, NewSet(), expectedWoBlacklisted},
		[]interface{}{commonMapStr(), "pre", emptyBlacklist, NewSet(), expectedAllPrefixed},
		[]interface{}{commonMapStr(), "", blacklist, NewSet("prefilled"), expectedWithFilledInput},
	}
	for idx, dataRow := range data {
		m := dataRow[mapDataIdx].(common.MapStr)
		prefix := dataRow[prefixIdx].(string)
		blacklist := dataRow[blacklistedIdx].(*Set)
		flattened := dataRow[inputIdx].(*Set)

		flattenMapStr(m, prefix, blacklist, flattened)
		expected := dataRow[retValIdx].(*Set)
		diff := SymmDifference(flattened, expected)

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
	assert.NotNil(t, err)

	fields, err := loadFields("./_meta/fields.yml")
	assert.Nil(t, err)
	expected := NewSet("transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace")
	flattened := NewSet()
	flattenFieldNames(fields, "", addAllFields, flattened)
	assert.Equal(t, expected, flattened)
}

func TestFlattenFieldNames(t *testing.T) {

	fields, _ := loadFields("./_meta/fields.yml")

	expected := NewSet("transaction", "transaction.id", "transaction.context", "exception", "exception.http", "exception.http.url", "exception.http.meta", "exception.stacktrace")

	flattened := NewSet()
	flattenFieldNames(fields, "", addAllFields, flattened)
	assert.Equal(t, expected, flattened)

	flattened = NewSet()
	flattenFieldNames(fields, "", addOnlyDisabledFields, flattened)
	expected = NewSet("transaction.context", "exception.stacktrace")
	assert.Equal(t, expected, flattened)
}
