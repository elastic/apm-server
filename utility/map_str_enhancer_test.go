package utility

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const addKey = "added"

func TestAdd(t *testing.T) {
	base := common.MapStr{"existing": "foo"}
	addTrue, updateTrue, expectedTrue := true, false, common.MapStr{"existing": "foo", addKey: true}
	addFalse, updateFalse, expectedFalse := false, true, common.MapStr{"existing": "foo", addKey: false}
	addInt, updateInt, expectedInt := 1, 2, common.MapStr{"existing": "foo", addKey: 1}
	addStr, updateStr, expectedStr := "foo", "bar", common.MapStr{"existing": "foo", addKey: "foo"}
	addCommonMapStr, updateCommonMapStr, expectedCommonMapStr := common.MapStr{"foo": "bar"}, common.MapStr{"john": "doe"}, common.MapStr{"existing": "foo", addKey: common.MapStr{"foo": "bar"}}
	addCommonMapStrEmpty, updateCommonMapStrEmpty, expectedCommonMapStrEmpty := common.MapStr{}, common.MapStr{}, base

	var addBoolNil *bool
	var addIntNil *int
	var addStrNil *string
	var addStrArrNil []string
	testData := [][]interface{}{
		{&addTrue, &updateTrue, expectedTrue},
		{&addFalse, &updateFalse, expectedFalse},
		{&addInt, &updateInt, expectedInt},
		{&addStr, &updateStr, expectedStr},
		{addCommonMapStr, updateCommonMapStr, expectedCommonMapStr},
		{addCommonMapStrEmpty, updateCommonMapStrEmpty, expectedCommonMapStrEmpty},
		{addBoolNil, addBoolNil, base},
		{addIntNil, addIntNil, base},
		{addStrNil, addStrNil, base},
		{addStrArrNil, []string{"something"}, base},
	}
	for _, testDataRow := range testData {
		base := baseMapStr()

		Add(base, addKey, testDataRow[0])
		assert.Equal(t, testDataRow[2], base)
	}
}

func TestStringWithDefault(t *testing.T) {
	base := baseMapStr()
	add := "foo"
	newMap := common.MapStr{"existing": "foo", "added": "foo"}
	AddStrWithDefault(base, addKey, &add, "bar")
	assert.Equal(t, newMap, base)

	base = baseMapStr()
	AddStrWithDefault(base, addKey, nil, "bar")
	newMap = common.MapStr{"existing": "foo", "added": "bar"}
	assert.Equal(t, newMap, base)
}

func baseMapStr() common.MapStr {
	return common.MapStr{"existing": "foo"}
}
