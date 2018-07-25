package beater

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNDJSONStreamReader(t *testing.T) {
	lines := []string{
		`{"key": "value1"}`,
		`{"key": "value2"}`,
		`{invalid-json}`,
		`{"key": "value3"}`,
	}
	expected := []struct {
		errPattern string
		out        map[string]interface{}
		isEOF      bool
	}{
		{
			out: map[string]interface{}{"key": "value1"},
		},
		{
			out: map[string]interface{}{"key": "value2"},
		},
		{
			out:        nil,
			errPattern: "invalid character",
		},
		{
			out:        map[string]interface{}{"key": "value3"},
			errPattern: "EOF",
			isEOF:      true,
		},
	}
	buf := bytes.NewBufferString(strings.Join(lines, "\n"))
	n := NDJSONStreamReader{stream: bufio.NewReader(buf)}

	for idx, test := range expected {
		out, err := n.Read()
		assert.Equal(t, test.out, out, "Failed at idx %v", idx)
		if test.errPattern == "" {
			assert.Nil(t, err)
		} else {
			assert.Contains(t, err.Error(), test.errPattern, "Failed at idx %v", idx)
		}
	}
}
