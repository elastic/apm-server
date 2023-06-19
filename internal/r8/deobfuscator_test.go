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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
)

type FrameValidation struct {
	updated           bool
	originalClassname string
	originalFunction  string
	classname         string
	function          string
	lineno            uint32
}

func TestSimpleLineDeobfuscation(t *testing.T) {
	// Input:
	// at androidx.appcompat.view.menu.e.f(Unknown Source:4)

	// Expected output:
	// at androidx.appcompat.view.menu.MenuBuilder.dispatchMenuItemSelected(Unknown Source:4)
	frame := createStacktraceFrame(4, "Unknown Source", "androidx.appcompat.view.menu.e", "f")
	stacktrace := []*modelpb.StacktraceFrame{frame}

	mapFilePath := "../../testdata/r8/deobfuscator/1/mapping"
	reader, err := os.Open(mapFilePath)
	require.NoError(t, err)
	defer reader.Close()

	err = Deobfuscate(&stacktrace, reader)
	require.NoError(t, err)

	verifyFrame(t, frame, FrameValidation{
		updated:           true,
		classname:         "androidx.appcompat.view.menu.MenuBuilder",
		function:          "dispatchMenuItemSelected",
		originalClassname: "androidx.appcompat.view.menu.e",
		originalFunction:  "f",
		lineno:            4,
	})
}

func TestDeobfuscateCompressedLine(t *testing.T) {
	// Input:
	// at i6.f.a(SourceFile:11)

	// Expected output:
	// at com.google.android.material.navigation.NavigationBarView$1.co.elastic.apm.opbeans.HomeActivity.oops(SourceFile:11)
	//																				co.elastic.apm.opbeans.HomeActivity.setUpBottomNavigation$lambda-0
	//																				onMenuItemSelected
	frame := createStacktraceFrame(11, "SourceFile", "i6.f", "a")
	stacktrace := []*modelpb.StacktraceFrame{frame}

	mapFilePath := "../../testdata/r8/deobfuscator/1/mapping"
	reader, err := os.Open(mapFilePath)
	require.Nil(t, err)
	defer reader.Close()

	err = Deobfuscate(&stacktrace, reader)
	require.Nil(t, err)

	verifyFrame(t, frame, FrameValidation{
		updated:           true,
		originalClassname: "i6.f",
		originalFunction:  "a",
		classname:         "com.google.android.material.navigation.NavigationBarView$1",
		function: `co.elastic.apm.opbeans.HomeActivity.oops
co.elastic.apm.opbeans.HomeActivity.setUpBottomNavigation$lambda-0
onMenuItemSelected`,
		lineno: 11,
	})
}

func TestDeobfuscateClassNameOnlyWhenMethodIsNotObfuscated(t *testing.T) {
	// Input:
	// at d.e.onStart(Unknown Source:0)

	// Expected output:
	// at androidx.appcompat.app.AppCompatActivity.onStart(Unknown Source:0)
	frame := createStacktraceFrame(0, "Unknown Source", "d.e", "onStart")
	stacktrace := []*modelpb.StacktraceFrame{frame}

	mapFilePath := "../../testdata/r8/deobfuscator/2/mapping"
	reader, err := os.Open(mapFilePath)
	require.Nil(t, err)
	defer reader.Close()

	err = Deobfuscate(&stacktrace, reader)
	require.Nil(t, err)

	verifyFrame(t, frame, FrameValidation{
		updated:           true,
		originalClassname: "d.e",
		classname:         "androidx.appcompat.app.AppCompatActivity",
		function:          "onStart",
	})
}

func TestDeobfuscateMultipleLines(t *testing.T) {
	// Input:
	//at java.lang.Class.getMethod(Class.java:2103)
	//at java.lang.Class.getMethod(Class.java:1724)
	//at m1.b.e(Unknown Source:32)
	//at n8.h.c(SourceFile:30)
	//at n8.d.a(SourceFile:50)
	//at n8.a.intercept(SourceFile:11)
	//at o8.f.b(SourceFile:7)
	//at l8.a.intercept(SourceFile:29)
	//at o8.f.b(SourceFile:7)
	//at o8.a.intercept(SourceFile:21)
	//at o8.f.b(SourceFile:7)
	//at o8.h.intercept(SourceFile:25)
	//at o8.f.b(SourceFile:7)
	//at b3.a.intercept(SourceFile:5)

	// Expected output:
	//at java.lang.Class.getMethod(Class.java:2103)
	//at java.lang.Class.getMethod(Class.java:1724)
	//at co.elastic.apm.android.common.okhttp.eventlistener.Generated_CompositeEventListener.connectFailed(Unknown Source:32)
	//at okhttp3.internal.connection.RealConnection.connect(SourceFile:30)
	//at okhttp3.internal.connection.ExchangeFinder.okhttp3.internal.connection.ExchangeFinder.findConnection(SourceFile:50)
	//												findHealthyConnection
	//at okhttp3.internal.connection.ConnectInterceptor.okhttp3.internal.connection.ExchangeFinder.find(SourceFile:11)
	//													okhttp3.internal.connection.RealCall.initExchange$okhttp
	//													intercept
	//at okhttp3.internal.http.RealInterceptorChain.proceed(SourceFile:7)
	//at okhttp3.internal.cache.CacheInterceptor.intercept(SourceFile:29)
	//at okhttp3.internal.http.RealInterceptorChain.proceed(SourceFile:7)
	//at okhttp3.internal.http.BridgeInterceptor.intercept(SourceFile:21)
	//at okhttp3.internal.http.RealInterceptorChain.proceed(SourceFile:7)
	//at okhttp3.internal.http.RetryAndFollowUpInterceptor.intercept(SourceFile:25)
	//at okhttp3.internal.http.RealInterceptorChain.proceed(SourceFile:7)
	//at co.elastic.apm.opbeans.app.di.ApplicationModule$provideOkHttpClient$$inlined$-addInterceptor$1.intercept(SourceFile:5)

	stacktrace := &[]*modelpb.StacktraceFrame{
		createStacktraceFrame(2103, "Class.java", "java.lang.Class", "getMethod"),
		createStacktraceFrame(1724, "Class.java", "java.lang.Class", "getMethod"),
		createStacktraceFrame(32, "Unknown Source", "m1.b", "e"),
		createStacktraceFrame(30, "SourceFile", "n8.h", "c"),
		createStacktraceFrame(50, "SourceFile", "n8.d", "a"),
		createStacktraceFrame(11, "SourceFile", "n8.a", "intercept"),
		createStacktraceFrame(7, "SourceFile", "o8.f", "b"),
		createStacktraceFrame(29, "SourceFile", "l8.a", "intercept"),
		createStacktraceFrame(7, "SourceFile", "o8.f", "b"),
		createStacktraceFrame(21, "SourceFile", "o8.a", "intercept"),
		createStacktraceFrame(7, "SourceFile", "o8.f", "b"),
		createStacktraceFrame(25, "SourceFile", "o8.h", "intercept"),
		createStacktraceFrame(7, "SourceFile", "o8.f", "b"),
		createStacktraceFrame(5, "SourceFile", "b3.a", "intercept"),
	}

	mapFilePath := "../../testdata/r8/deobfuscator/3/mapping"
	reader, err := os.Open(mapFilePath)
	require.Nil(t, err)
	defer reader.Close()

	err = Deobfuscate(stacktrace, reader)
	require.Nil(t, err)

	verifyFrames(t, *stacktrace, []FrameValidation{
		{function: "getMethod", classname: "java.lang.Class", lineno: 2103},
		{function: "getMethod", classname: "java.lang.Class", lineno: 1724},
		{
			updated:           true,
			function:          "connectFailed",
			classname:         "co.elastic.apm.android.common.okhttp.eventlistener.Generated_CompositeEventListener",
			originalFunction:  "e",
			originalClassname: "m1.b",
			lineno:            32,
		},
		{
			updated:           true,
			function:          "connect",
			classname:         "okhttp3.internal.connection.RealConnection",
			originalFunction:  "c",
			originalClassname: "n8.h",
			lineno:            30,
		},
		{
			updated: true,
			function: `okhttp3.internal.connection.ExchangeFinder.findConnection
findHealthyConnection`,
			classname:         "okhttp3.internal.connection.ExchangeFinder",
			originalFunction:  "a",
			originalClassname: "n8.d",
			lineno:            50,
		},
		{
			updated: true,
			function: `okhttp3.internal.connection.ExchangeFinder.find
okhttp3.internal.connection.RealCall.initExchange$okhttp
intercept`,
			classname:         "okhttp3.internal.connection.ConnectInterceptor",
			originalFunction:  "intercept",
			originalClassname: "n8.a",
			lineno:            11,
		},
		{
			updated:           true,
			function:          "proceed",
			classname:         "okhttp3.internal.http.RealInterceptorChain",
			originalFunction:  "b",
			originalClassname: "o8.f",
			lineno:            7,
		},
		{
			updated:           true,
			function:          "intercept",
			classname:         "okhttp3.internal.cache.CacheInterceptor",
			originalFunction:  "intercept",
			originalClassname: "l8.a",
			lineno:            29,
		},
		{
			updated:           true,
			function:          "proceed",
			classname:         "okhttp3.internal.http.RealInterceptorChain",
			originalFunction:  "b",
			originalClassname: "o8.f",
			lineno:            7,
		},
		{
			updated:           true,
			function:          "intercept",
			classname:         "okhttp3.internal.http.BridgeInterceptor",
			originalFunction:  "intercept",
			originalClassname: "o8.a",
			lineno:            21,
		},
		{
			updated:           true,
			function:          "proceed",
			classname:         "okhttp3.internal.http.RealInterceptorChain",
			originalFunction:  "b",
			originalClassname: "o8.f",
			lineno:            7,
		},
		{
			updated:           true,
			function:          "intercept",
			classname:         "okhttp3.internal.http.RetryAndFollowUpInterceptor",
			originalFunction:  "intercept",
			originalClassname: "o8.h",
			lineno:            25,
		},
		{
			updated:           true,
			function:          "proceed",
			classname:         "okhttp3.internal.http.RealInterceptorChain",
			originalFunction:  "b",
			originalClassname: "o8.f",
			lineno:            7,
		},
		{
			updated:           true,
			function:          "intercept",
			classname:         "co.elastic.apm.opbeans.app.di.ApplicationModule$provideOkHttpClient$$inlined$-addInterceptor$1",
			originalFunction:  "intercept",
			originalClassname: "b3.a",
			lineno:            5,
		},
	})
}

func verifyFrames(t *testing.T, frames []*modelpb.StacktraceFrame, validations []FrameValidation) {
	assert.Equal(t, len(validations), len(frames))

	for index, frame := range frames {
		verifyFrame(t, frame, validations[index])
	}
}

func verifyFrame(t *testing.T, frame *modelpb.StacktraceFrame, validation FrameValidation) {
	assert.Equal(t, validation.updated, frame.SourcemapUpdated)
	assert.Equal(t, validation.originalClassname, frame.GetOriginal().GetClassname())
	assert.Equal(t, validation.originalFunction, frame.GetOriginal().GetFunction())
	if frame.Original != nil {
		assert.Nil(t, frame.Original.Lineno)
	}
	assert.Equal(t, validation.classname, frame.Classname)
	assert.Equal(t, validation.function, frame.Function)
	assert.Equal(t, validation.lineno, *frame.Lineno)
}

func createStacktraceFrame(lineno uint32, fileName string, className string, function string) *modelpb.StacktraceFrame {
	frame := modelpb.StacktraceFrame{
		Lineno:    &lineno,
		Filename:  fileName,
		Classname: className,
		Function:  function,
	}
	return &frame
}
