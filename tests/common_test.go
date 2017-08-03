package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
