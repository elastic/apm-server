package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDifferenceWithRegex(t *testing.T) {
	for idx, d := range []struct {
		s1, s2, diff *Set
	}{
		{nil, nil, nil},
		{NewSet("a.b.c", 2, 3), nil, NewSet("a.b.c", 2, 3)},
		{nil, NewSet("a.b.c", 2, 3), nil},
		{NewSet("a.b.c", 2), NewSet(2), NewSet("a.b.c")},
		{NewSet("a.b.c", 2), NewSet(Group("b")), NewSet("a.b.c", 2)},
		{NewSet("a.b.c", "ab", "a.c", "a.b.", 3), NewSet(Group("a.b.")), NewSet("ab", "a.c", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet(Group("a.b")), NewSet("a", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet("a.b"), NewSet("a", "a.b.c", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet("a.b*"), NewSet("a", "a.b", "a.b.c", 3)},
	} {
		out := differenceWithGroup(d.s1, d.s2)
		assert.ElementsMatch(t, d.diff.Array(), out.Array(),
			fmt.Sprintf("Idx <%v>: Expected %v, Actual %v", idx, d.diff.Array(), out.Array()))
	}

}

func TestStrConcat(t *testing.T) {
	preIdx, postIdx, delimiterIdx, expectedIdx := 0, 1, 2, 3
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
		newStr := strConcat(dataRow[preIdx], dataRow[postIdx], dataRow[delimiterIdx])
		assert.Equal(t, dataRow[expectedIdx], newStr)
	}
}
