package tests

import (
	"fmt"
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const mapDataIdx = 0
const prefixIdx = 1
const blacklistedIdx = 2
const inputIdx = 3
const retValIdx = 4

func TestFlattenCommonMapStr(t *testing.T) {
	emptyBlacklist := set.New()
	blacklist := set.New("a.bMap", "f")
	expectedAll := set.New("a", "a.bStr", "a.bMap", "a.bMap.cMap", "a.bMap.cMap.d", "a.bMap.cStr", "a.bAnotherMap", "a.bAnotherMap.e", "f")
	expectedWoBlacklisted := set.New("a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e")
	expectedAllPrefixed := set.New("pre", "pre.a", "pre.a.bStr", "pre.a.bMap", "pre.a.bMap.cMap", "pre.a.bMap.cMap.d", "pre.a.bMap.cStr", "pre.a.bAnotherMap", "pre.a.bAnotherMap.e", "pre.f")
	expectedWithFilledInput := set.New("prefilled", "a", "a.bStr", "a.bAnotherMap", "a.bAnotherMap.e")
	data := [][]interface{}{
		[]interface{}{common.MapStr{}, "whatever", emptyBlacklist, set.New(), set.New("whatever")},
		[]interface{}{common.MapStr{}, "", blacklist, set.New(), set.New()},
		[]interface{}{commonMapStr(), "", emptyBlacklist, set.New(), expectedAll},
		[]interface{}{commonMapStr(), "", blacklist, set.New(), expectedWoBlacklisted},
		[]interface{}{commonMapStr(), "pre", emptyBlacklist, set.New(), expectedAllPrefixed},
		[]interface{}{commonMapStr(), "", blacklist, set.New("prefilled"), expectedWithFilledInput},
	}
	for idx, dataRow := range data {
		m := dataRow[mapDataIdx].(common.MapStr)
		prefix := dataRow[prefixIdx].(string)
		blacklist := dataRow[blacklistedIdx].(*set.Set)
		flattened := dataRow[inputIdx].(*set.Set)

		flattenMapStr(m, prefix, blacklist, flattened)
		expected := dataRow[retValIdx].(*set.Set)
		diff := set.SymmetricDifference(flattened, expected).(*set.Set)

		errMsg := fmt.Sprintf("Failed for idx %v, diff: %v", idx, diff)
		assert.Equal(t, 0, diff.Size(), errMsg)
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
