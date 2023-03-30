package r8

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
		reader, err := os.Open(mapFilePath)
		if err != nil {
			log.Fatal(err)
		}

		t.Run(fmt.Sprintf("(%s)->(%s)", inputPath, expectedOutputPath), func(t *testing.T) {
			obfuscated := readFile(inputPath)
			expected := readFile(expectedOutputPath)
			deobfuscated, _ := Deobfuscate(obfuscated, reader)
			assert.Equal(t, expected, deobfuscated)
			err := reader.Close()
			if err != nil {
				log.Fatal(err)
			}
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
