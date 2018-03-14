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
