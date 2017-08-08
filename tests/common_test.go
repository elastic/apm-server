package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestFlattenJsonKeys(t *testing.T) {
	flattened := []string{}
	s := `{
		"app": { "name": "john", "age": 25 },
		"transactions": [
			{ "id": "123" },
			{ "id": "555" }
		]
	}`
	var data interface{}
	json.NewDecoder(strings.NewReader(s)).Decode(&data)
	dataFlattened := []string{"-.app", "-.app.name", "-.app.age", "-.transactions", "-.transactions.id", "-.transactions.id"}
	testData := [][]interface{}{
		{map[string]interface{}{}, "", &flattened, flattened},
		{data, "-", &flattened, dataFlattened},
	}
	for _, d := range testData {
		keys := d[2].(*[]string)
		FlattenJsonKeys(d[0], d[1].(string), keys)
		expected := d[3].([]string)
		diff, _ := ArrayDiff(expected, *keys)
		assert.Equal(t, 0, len(diff))
	}
}

func TestArrayDiff(t *testing.T) {
	testData := [][]interface{}{
		[]interface{}{[]string{""}, []string{""}, []string{}},
		[]interface{}{[]string{}, []string{}, []string{}},
		[]interface{}{[]string{"a"}, []string{"a"}, []string{}},
		[]interface{}{[]string{"", "a"}, []string{"a"}, []string{""}},
		[]interface{}{[]string{"a", "b"}, []string{"a"}, []string{"b"}},
		[]interface{}{[]string{"a", "a"}, []string{"a"}, []string{}},
		[]interface{}{[]string{"a"}, []string{"a", "b"}, []string{}},
		[]interface{}{[]string{}, []string{"a", "b"}, []string{}},
	}
	for _, data := range testData {
		a := data[0].([]string)
		b := data[1].([]string)
		expectedDiff := data[2].([]string)
		diff, _ := ArrayDiff(a, b)
		assert.Equal(t, expectedDiff, diff)
	}
}

const preIdx = 0
const postIdx = 1
const delimiterIdx = 2
const expectedIdx = 3

func TestStrConcat(t *testing.T) {
	testData := [][]string{
		[]string{"", "", "", ""},
		[]string{"pre", "", "", "pre"},
		[]string{"pre", "post", "", "prepost"},
		[]string{"foo", "bar", ".", "foo.bar"},
		[]string{"foo", "bar", ":", "foo:bar"},
		[]string{"", "post", "", "post"},
		[]string{"", "post", ",", "post"},
	}
	for _, dataRow := range testData {
		newStr := StrConcat(dataRow[preIdx], dataRow[postIdx], dataRow[delimiterIdx])
		assert.Equal(t, dataRow[expectedIdx], newStr)
	}
}

const strIdx = 0
const sliceIdx = 1
const includedIdx = 2

func TestStrIdxInSlice(t *testing.T) {
	testData := [][]interface{}{
		[]interface{}{"", []string{}, -1},
		[]interface{}{"", []string{""}, 0},
		[]interface{}{"", []string{"some"}, -1},
		[]interface{}{"some", []string{}, -1},
		[]interface{}{"some", []string{"some"}, 0},
		[]interface{}{"some", []string{"foo", "some", "bar"}, 1},
		[]interface{}{"some", []string{"foo", "something", "bar"}, -1},
	}
	for _, dataRow := range testData {
		slice := dataRow[sliceIdx].([]string)
		strInSlice := strIdxInSlice(dataRow[strIdx].(string), slice)
		assert.Equal(t, dataRow[includedIdx], strInSlice)
	}

}

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

		FlattenMapStr(m, prefix, blacklist, flattened)
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
