package r8

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func TestDeobfuscation(t *testing.T) {
	cases := []string{
		"../../testdata/r8/deobfuscator/1",
		"../../testdata/r8/deobfuscator/2",
		"../../testdata/r8/deobfuscator/3",
	}

	for _, c := range cases {
		inputPath := c + "/obfuscated-crash"
		expectedOutputPath := c + "/de-obfuscated-crash"
		mapFilePath := c + "/mapping"
		t.Run(fmt.Sprintf("(%s)->(%s)", inputPath, expectedOutputPath), func(t *testing.T) {
			obfuscated := readFile(inputPath)
			expected := readFile(expectedOutputPath)
			deobfuscated, _ := Deobfuscate(obfuscated, mapFilePath)
			assert.Equal(t, expected, deobfuscated)
		})
	}
}

func readFile(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return string(bytes)
}
