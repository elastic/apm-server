package tests

import (
	"sort"
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
	emptyBlacklist := []string{}
	blacklist := []string{"a.bMap", "f"}
	expectedAll := []string{"a", "a.bStr", "a.bMap", "a.bMap.cMap", "a.bMap.cMap.d", "a.bMap.cStr", "a.bAnotherMap", "a.bAnotherMap.e", "f"}
	expectedWoBlacklisted := []string{"a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e"}
	expectedAllPrefixed := []string{"pre.a", "pre.a.bStr", "pre.a.bMap", "pre.a.bMap.cMap", "pre.a.bMap.cMap.d", "pre.a.bMap.cStr", "pre.a.bAnotherMap", "pre.a.bAnotherMap.e", "pre.f"}
	expectedWithFilledInput := []string{"prefilled", "a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e"}
	data := [][]interface{}{
		[]interface{}{common.MapStr{}, "whatever", emptyBlacklist, []string{}, []string{}},
		[]interface{}{common.MapStr{}, "", blacklist, []string{}, []string{}},
		[]interface{}{commonMapStr(), "", emptyBlacklist, []string{}, expectedAll},
		[]interface{}{commonMapStr(), "", blacklist, []string{}, expectedWoBlacklisted},
		[]interface{}{commonMapStr(), "pre", emptyBlacklist, []string{}, expectedAllPrefixed},
		[]interface{}{commonMapStr(), "", blacklist, []string{"prefilled"}, expectedWithFilledInput},
	}
	for _, dataRow := range data {
		m := dataRow[mapDataIdx].(common.MapStr)
		prefix := dataRow[prefixIdx].(string)
		blacklist := dataRow[blacklistedIdx].([]string)
		flattened := dataRow[inputIdx].([]string)

		result := FlattenCommonMapStr(m, prefix, blacklist, flattened)
		sort.Strings(result)
		expected := dataRow[retValIdx].([]string)
		sort.Strings(expected)

		assert.Equal(t, expected, result)
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
			"bAnotherMap": common.MapStr{
				"e": 1,
			},
		},
		"f": "",
	}
}
