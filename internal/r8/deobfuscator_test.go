// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
