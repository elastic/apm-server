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
	// Input:
	// at androidx.appcompat.view.menu.e.f(Unknown Source:4)

	// Expected output:
	// at androidx.appcompat.view.menu.MenuBuilder.dispatchMenuItemSelected(Unknown Source:4)
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

	assert.Equal(t, true, frame.SourcemapUpdated)
	assert.Equal(t, "androidx.appcompat.view.menu.MenuBuilder", frame.Classname)
	assert.Equal(t, "dispatchMenuItemSelected", frame.Function)
	assert.Equal(t, "androidx.appcompat.view.menu.e", frame.Original.Classname)
	assert.Equal(t, "f", frame.Original.Function)
}

func TestDeobfuscateCompressedLine(t *testing.T) {
	// Input:
	// at i6.f.a(SourceFile:11)

	// Expected output:
	// at com.google.android.material.navigation.NavigationBarView$1.co.elastic.apm.opbeans.HomeActivity.oops(SourceFile:11)
	//																				co.elastic.apm.opbeans.HomeActivity.setUpBottomNavigation$lambda-0
	//																				onMenuItemSelected
	lineno := 11
	frame := model.StacktraceFrame{
		Lineno:    &lineno,
		Filename:  "SourceFile",
		Classname: "i6.f",
		Function:  "a",
	}
	var stacktrace = new(model.Stacktrace)
	*stacktrace = append(*stacktrace, &frame)

	mapFilePath := "../../testdata/r8/deobfuscator/1/mapping"
	reader, err := os.Open(mapFilePath)
	require.Nil(t, err)
	defer reader.Close()

	err = Deobfuscate(stacktrace, reader)
	require.Nil(t, err)

	assert.Equal(t, true, frame.SourcemapUpdated)
	assert.Equal(t, "i6.f", frame.Original.Classname)
	assert.Equal(t, "a", frame.Original.Function)
	assert.Equal(t, "com.google.android.material.navigation.NavigationBarView$1", frame.Classname)
	assert.Equal(t, `co.elastic.apm.opbeans.HomeActivity.oops
co.elastic.apm.opbeans.HomeActivity.setUpBottomNavigation$lambda-0
onMenuItemSelected`, frame.Function)
}
