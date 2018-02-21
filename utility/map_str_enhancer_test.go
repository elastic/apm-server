package utility

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

const addKey = "added"

func TestAddGeneral(t *testing.T) {
	var m common.MapStr
	Add(m, "s", "s")
	assert.Nil(t, m)

	m = common.MapStr{}
	Add(m, "", "")
	assert.Equal(t, common.MapStr{}, m)
}

func TestEmptyCollections(t *testing.T) {
	m := common.MapStr{"foo": "bar"}
	add := common.MapStr{}
	Add(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)

	m = common.MapStr{"foo": "bar"}
	add = map[string]interface{}{}
	Add(m, "user", add)
	assert.Equal(t, common.MapStr{"foo": "bar"}, m)
}

func TestAdd(t *testing.T) {
	existing := "foo"
	newArrMapStr := []common.MapStr{{"b": "bar"}}
	var nilArrMapStr []common.MapStr

	newArrStr := []string{"bar"}
	var nilArrStr []string

	newMap := map[string]interface{}{"b": "bar"}
	var nilMap map[string]interface{}

	newMapStr := common.MapStr{"b": "bar"}
	var nilMapStr common.MapStr

	newStr := "bar"
	var nilStr *string

	newInt := 123
	var nilInt *int

	newBool := true
	var nilBool *bool

	tests := []struct {
		v    interface{}
		expV interface{}
		nilV interface{}
	}{
		{
			v:    "some string",
			expV: "some string",
			nilV: nil,
		},
		{
			v:    &newBool,
			expV: newBool,
			nilV: nilBool,
		},
		{
			v:    &newInt,
			expV: newInt,
			nilV: nilInt,
		},
		{
			v:    &newStr,
			expV: newStr,
			nilV: nilStr,
		},
		{
			v:    newMapStr,
			expV: newMapStr,
			nilV: nilMapStr,
		},
		{
			v:    newMap,
			expV: newMap,
			nilV: nilMap,
		},
		{
			v:    newArrStr,
			expV: newArrStr,
			nilV: nilArrStr,
		},
		{
			v:    newArrMapStr,
			expV: newArrMapStr,
			nilV: nilArrMapStr,
		},
	}

	for idx, te := range tests {
		// add new value
		m := common.MapStr{"existing": existing}
		Add(m, addKey, te.v)
		expected := common.MapStr{"existing": existing, addKey: te.expV}
		assert.Equal(t, expected, m,
			fmt.Sprintf("<%v>: Add new value - Expected: %v, Actual: %v", idx, expected, m))

		// replace existing value
		m = common.MapStr{addKey: existing}
		Add(m, addKey, te.v)
		expected = common.MapStr{addKey: te.expV}
		assert.Equal(t, expected, m,
			fmt.Sprintf("<%v>: Replace existing value - Expected: %v, Actual: %v", idx, expected, m))

	}
}

func TestMergeAddCommonMapStr(t *testing.T) {
	type M = common.MapStr
	testData := []struct {
		data   M
		key    string
		values M
		result M
	}{
		{
			//map is nil
			nil,
			"a",
			M{"a": 1},
			nil,
		},
		{
			//key is nil
			M{"a": 1},
			"",
			M{"a": 2},
			M{"a": 1},
		},
		{
			//val is nil
			M{"a": 1},
			"a",
			nil,
			M{"a": 1},
		},
		{
			//val is empty
			M{"a": 1},
			"a",
			M{},
			M{"a": 1},
		},
		{
			M{"a": 1},
			"b",
			M{"c": 2},
			M{"a": 1, "b": M{"c": 2}},
		},
		{
			M{"a": 1},
			"a",
			M{"b": 2},
			M{"a": 1},
		},
		{
			M{"a": M{"b": 1}},
			"a",
			M{"c": 2},
			M{"a": M{"b": 1, "c": 2}},
		},
	}
	for idx, test := range testData {
		MergeAdd(test.data, test.key, test.values)
		assert.Equal(t, test.result, test.data,
			fmt.Sprintf("At (%v): Expected %s, got %s", idx, test.result, test.data))
	}
}

func TestMergeAddMap(t *testing.T) {
	type M = map[string]interface{}
	type CM = common.MapStr
	testData := []struct {
		data   M
		key    string
		values CM
		result M
	}{
		{
			//map is nil
			nil,
			"a",
			M{"a": 1},
			nil,
		},
		{
			//key is nil
			M{"a": 1},
			"",
			M{"a": 2},
			M{"a": 1},
		},
		{
			//val is nil
			M{"a": 1},
			"a",
			nil,
			M{"a": 1},
		},
		{
			//val is empty
			M{"a": 1},
			"a",
			CM{},
			M{"a": 1},
		},
		{
			M{"a": 1},
			"b",
			CM{"c": 2},
			M{"a": 1, "b": CM{"c": 2}},
		},
		{
			M{"a": 1},
			"a",
			CM{"b": 2},
			M{"a": 1},
		},
		{
			M{"a": M{"b": 1}},
			"a",
			CM{"c": 2},
			M{"a": M{"b": 1, "c": 2}},
		},
	}
	for idx, test := range testData {
		MergeAdd(test.data, test.key, test.values)
		assert.Equal(t, test.result, test.data,
			fmt.Sprintf("At (%v): Expected %s, got %s", idx, test.result, test.data))
	}
}

func TestMillisAsMicros(t *testing.T) {
	ms := 4.5
	m := MillisAsMicros(ms)
	expectedMap := common.MapStr{"us": 4500}
	assert.Equal(t, expectedMap, m)
}

func TestStringPtrWithDefault(t *testing.T) {
	old := "foo"
	new := "bar"
	def := "def"

	// map is nil
	var m common.MapStr
	AddStrWithDefault(m, addKey, nil, def)
	assert.Nil(t, m)

	// key is empty
	m = common.MapStr{"existing": old}
	AddStrWithDefault(m, "", &new, def)
	assert.Equal(t, common.MapStr{"existing": old}, m)

	//sets the new value
	m = common.MapStr{"existing": old}
	AddStrWithDefault(m, addKey, &new, def)
	expectedMap := common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//overrides existing value
	m = common.MapStr{"existing": old, addKey: old}
	AddStrWithDefault(m, addKey, &new, def)
	expectedMap = common.MapStr{"existing": old, addKey: new}
	assert.Equal(t, expectedMap, m)

	//sets default value if no value given
	AddStrWithDefault(m, addKey, nil, def)
	expectedMap = common.MapStr{"existing": old, addKey: def}
	assert.Equal(t, expectedMap, m)
}
