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
	"os"
	"testing"

	"github.com/elastic/apm-data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleLineDeobfuscation(t *testing.T) {
	//at androidx.appcompat.view.menu.e.f(Unknown Source:4)
	lineno := 4
	frame := model.StacktraceFrame{
		Lineno:    &lineno,
		Filename:  "Unknown Source",
		Classname: "androidx.appcompat.view.menu.e",
		Function:  "f",
	}
	var stacktrace = new(model.Stacktrace)
	*stacktrace = append(*stacktrace, &frame)

	mapFilePath := "../../testdata/r8/deobfuscator/1/mapping"
	reader, err := os.Open(mapFilePath)
	require.Nil(t, err)
	defer reader.Close()

	err = Deobfuscate(stacktrace, reader)
	require.Nil(t, err)

	assert.Equal(t, "androidx.appcompat.view.menu.MenuBuilder", frame.Classname)
	assert.Equal(t, "androidx.appcompat.view.menu.e", frame.Original.Classname)
	assert.Equal(t, "dispatchMenuItemSelected", frame.Function)
	assert.Equal(t, "f", frame.Original.Function)
	assert.Equal(t, true, frame.SourcemapUpdated)
}

func TestDeobfuscation(t *testing.T) {
	//cases := []string{
	//	"../../testdata/r8/deobfuscator/1",
	//	"../../testdata/r8/deobfuscator/2",
	//	"../../testdata/r8/deobfuscator/3",
	//}
	//
	//for _, c := range cases {
	//	inputPath := c + "/obfuscated-crash"
	//	expectedOutputPath := c + "/de-obfuscated-crash"
	//	mapFilePath := c + "/mapping"
	//
	//	t.Run(fmt.Sprintf("(%s)->(%s)", inputPath, expectedOutputPath), func(t *testing.T) {
	//		t.Parallel()
	//		reader, err := os.Open(mapFilePath)
	//		require.Nil(t, err)
	//		defer reader.Close()
	//		obfuscated := readFile(t, inputPath)
	//		expected := readFile(t, expectedOutputPath)
	//		deobfuscated, _ := Deobfuscate(obfuscated, reader)
	//		assert.Equal(t, expected, deobfuscated)
	//	})
	//}
}

func readFile(t *testing.T, path string) string {
	bytes, err := os.ReadFile(path)
	require.Nil(t, err)
	return string(bytes)
}
