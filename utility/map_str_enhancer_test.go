package utility

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const addKey = "added"

//func TestAdd(t *testing.T) {
//base := common.MapStr{"existing": "foo"}
//addTrue, updateTrue, expectedTrue := true, false, common.MapStr{"existing": "foo", addKey: true}
//addFalse, updateFalse, expectedFalse := false, true, common.MapStr{"existing": "foo", addKey: false}
//addInt, updateInt, expectedInt := 1, 2, common.MapStr{"existing": "foo", addKey: 1}
//addStr, updateStr, expectedStr := "foo", "bar", common.MapStr{"existing": "foo", addKey: "foo"}
//addCommonMapStr, updateCommonMapStr, expectedCommonMapStr := common.MapStr{"foo": "bar"}, common.MapStr{"john": "doe"}, common.MapStr{"existing": "foo", addKey: common.MapStr{"foo": "bar"}}
//addCommonMapStrEmpty, updateCommonMapStrEmpty, expectedCommonMapStrEmpty := common.MapStr{}, common.MapStr{}, base

//var addBoolNil *bool
//var addIntNil *int
//var addStrNil *string
//var addStrArrNil []string
//testData := [][]interface{}{
//{&addTrue, &updateTrue, expectedTrue},
//{&addFalse, &updateFalse, expectedFalse},
//{&addInt, &updateInt, expectedInt},
//{&addStr, &updateStr, expectedStr},
//{addCommonMapStr, updateCommonMapStr, expectedCommonMapStr},
//{addCommonMapStrEmpty, updateCommonMapStrEmpty, expectedCommonMapStrEmpty},
//{addBoolNil, addBoolNil, base},
//{addIntNil, addIntNil, base},
//{addStrNil, addStrNil, base},
//{addStrArrNil, []string{"something"}, base},
//}
//for _, testDataRow := range testData {
//assert, enhancer, base := setup(t)

//enhancer.Add(base, addKey, testDataRow[0])
//assert.Equal(testDataRow[2], base)
//}
//}

func TestStringPtr(t *testing.T) {
	old := "foo"
	new := "bar"

	//sets the new value
	m := common.MapStr{"existing": &old}
	AddStrPtr(m, addKey, &new)
	expectedMap := common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": &old, addKey: &old}
	AddStrPtr(m, addKey, &new)
	expectedMap = common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)
}

func TestStringPtrWithDefault(t *testing.T) {
	old := "foo"
	new := "bar"
	def := "def"

	//sets the new value
	m := common.MapStr{"existing": &old}
	AddStrPtrWithDefault(m, addKey, &new, &def)
	expectedMap := common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": &old, addKey: &old}
	AddStrPtrWithDefault(m, addKey, &new, &def)
	expectedMap = common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)

	//sets default value if no value given
	AddStrPtrWithDefault(m, addKey, nil, &def)
	expectedMap = common.MapStr{"existing": &old, addKey: &def}
	assert.Equal(t, expectedMap, m)
}

func TestStringArray(t *testing.T) {
	old := []string{"foo"}
	new := []string{"bar"}

	//sets the new value
	m := common.MapStr{"existing": old}
	AddStrArray(m, addKey, new)
	expectedMap := common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": old, addKey: old}
	AddStrArray(m, addKey, new)
	expectedMap = common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)
}

func TestIntPtr(t *testing.T) {
	old := 123
	new := 456

	//sets the new value
	m := common.MapStr{"existing": &old}
	AddIntPtr(m, addKey, &new)
	expectedMap := common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": &old, addKey: &old}
	AddIntPtr(m, addKey, &new)
	expectedMap = common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)
}

func TestBoolPtr(t *testing.T) {
	old := false
	new := true

	//sets the new value
	m := common.MapStr{"existing": &old}
	AddBoolPtr(m, addKey, &new)
	expectedMap := common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": &old, addKey: &old}
	AddBoolPtr(m, addKey, &new)
	expectedMap = common.MapStr{"existing": &old, addKey: &new}
	assert.Equal(t, expectedMap, m)
}

func TestMillis(t *testing.T) {
	old, new := 1.3, 4.5
	newMicros := 4500

	//sets the new value
	m := common.MapStr{"existing": &old}
	AddMillis(m, addKey, &new)
	expectedMap := common.MapStr{"existing": &old, addKey: common.MapStr{"us": &newMicros}}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": &old, addKey: &old}
	AddMillis(m, addKey, &new)
	expectedMap = common.MapStr{"existing": &old, addKey: common.MapStr{"us": &newMicros}}
	assert.Equal(t, expectedMap, m)
}

func TestInterface(t *testing.T) {
	var old, new interface{}
	old, new = "foo", "bar"

	//sets the new value
	m := common.MapStr{"existing": old}
	AddInterface(m, addKey, new)
	expectedMap := common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": old, addKey: old}
	AddInterface(m, addKey, new)
	expectedMap = common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)
}

func TestAddCommonMapStr(t *testing.T) {
	old, new := common.MapStr{"a": "foo"}, common.MapStr{"b": "bar"}

	//sets the new value
	m := common.MapStr{"existing": old}
	AddCommonMapStr(m, addKey, new)
	expectedMap := common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": old, addKey: old}
	AddCommonMapStr(m, addKey, new)
	expectedMap = common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)
}

func TestAddCommonMapStrArray(t *testing.T) {
	old := []common.MapStr{{"a": "foo"}}
	new := []common.MapStr{{"b": "bar"}}

	//sets the new value
	m := common.MapStr{"existing": old}
	AddCommonMapStrArray(m, addKey, new)
	expectedMap := common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": old, addKey: old}
	AddCommonMapStrArray(m, addKey, new)
	expectedMap = common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)
}
