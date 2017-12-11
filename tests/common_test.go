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
		{"", "", "", ""},
		{"pre", "", "", "pre"},
		{"pre", "post", "", "prepost"},
		{"foo", "bar", ".", "foo.bar"},
		{"foo", "bar", ":", "foo:bar"},
		{"", "post", "", "post"},
		{"", "post", ",", "post"},
	}
	for _, dataRow := range testData {
		newStr := StrConcat(dataRow[preIdx], dataRow[postIdx], dataRow[delimiterIdx])
		assert.Equal(t, dataRow[expectedIdx], newStr)
	}
}
