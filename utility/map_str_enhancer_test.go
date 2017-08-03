package utility

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const addKey = "added"

func TestAdd(t *testing.T) {
	/*TODO: check if/how to test the error cases,
	  it looks like the common.MapStr.Put() cannot really send back an error
	*/

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
		[]interface{}{&addTrue, &updateTrue, expectedTrue},
		[]interface{}{&addFalse, &updateFalse, expectedFalse},
		[]interface{}{&addInt, &updateInt, expectedInt},
		[]interface{}{&addStr, &updateStr, expectedStr},
		[]interface{}{addCommonMapStr, updateCommonMapStr, expectedCommonMapStr},
		[]interface{}{addCommonMapStrEmpty, updateCommonMapStrEmpty, expectedCommonMapStrEmpty},
		[]interface{}{addBoolNil, addBoolNil, base},
		[]interface{}{addIntNil, addIntNil, base},
		[]interface{}{addStrNil, addStrNil, base},
		[]interface{}{addStrArrNil, []string{"something"}, base},
	}
	for _, testDataRow := range testData {
		assert, enhancer, base := setup(t)

		enhancer.Add(base, addKey, testDataRow[0])
		assert.Equal(testDataRow[2], base)
	}
}

func TestStringWithDefault(t *testing.T) {
	assert, enhancer, base := setup(t)

	add := "foo"
	newMap := common.MapStr{"existing": "foo", "added": "foo"}
	enhancer.AddStrWithDefault(base, addKey, &add, "bar")
	assert.Equal(newMap, base)

	assert, enhancer, base = setup(t)
	enhancer.AddStrWithDefault(base, addKey, nil, "bar")
	newMap = common.MapStr{"existing": "foo", "added": "bar"}
	assert.Equal(newMap, base)
}

func setup(t *testing.T) (*assert.Assertions, MapStrEnhancer, common.MapStr) {
	a := assert.New(t)
	enhancer := NewMapStrEnhancer()
	base := common.MapStr{"existing": "foo"}
	return a, enhancer, base
}
