package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
