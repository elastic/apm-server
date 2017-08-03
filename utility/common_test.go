package utility

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTime(t *testing.T) {

	testData := [][]string{
		[]string{"2017-05-09T15:04:05.999999Z", "2017-05-09 15:04:05.999999 +0000 UTC"},
		[]string{"2017-05-09T15:04:05.999Z", "2017-05-09 15:04:05.999 +0000 UTC"},
	}
	for _, dataRow := range testData {
		parsedT, err := ParseTime(dataRow[0])
		assert.Equal(t, dataRow[1], parsedT.String())
		assert.Nil(t, err)
	}

	for _, invalidStr := range []string{"", "something", "2017-05-09", "2017-05-09T15:04:05+001"} {
		_, err := ParseTime(invalidStr)
		assert.NotNil(t, err)

	}
}
