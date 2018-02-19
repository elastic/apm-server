package utility

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUrlPath(t *testing.T) {
	testData := []struct {
		Url        string
		CleanedUrl string
	}{
		{Url: "!@#$%ˆ&*()", CleanedUrl: "!@#$%ˆ&*()"}, //leads to parse error
		{Url: "", CleanedUrl: "."},
		{Url: "a/c", CleanedUrl: "a/c"},
		{Url: "a//c", CleanedUrl: "a/c"},
		{Url: "a/c/.", CleanedUrl: "a/c"},
		{Url: "a/c/b/..", CleanedUrl: "a/c"},
		{Url: "/../a/c", CleanedUrl: "/a/c"},
		{Url: "/../a/b/../././/c", CleanedUrl: "/a/c"},
	}
	for idx, test := range testData {
		cleanedUrl := CleanUrlPath(test.Url)
		assert.Equal(t, test.CleanedUrl, cleanedUrl, fmt.Sprintf("(%v): Expected %s, got %s", idx, test.CleanedUrl, cleanedUrl))
	}
}

func TestInsertInMap(t *testing.T) {
	type M = map[string]interface{}
	testData := []struct {
		data   M
		key    string
		values M
		result M
	}{
		{
			nil,
			"a",
			M{"a": 1},
			nil,
		},
		{
			M{"a": 1},
			"",
			nil,
			M{"a": 1},
		},
		{
			M{"a": 1},
			"",
			M{},
			M{"a": 1},
		},
		{
			M{},
			"",
			M{"a": 1},
			M{},
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
		InsertInMap(test.data, test.key, test.values)
		assert.Equal(t, test.result, test.data,
			fmt.Sprintf("At (%v): Expected %s, got %s", idx, test.result, test.data))
	}
}
