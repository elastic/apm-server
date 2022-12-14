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

// go:build windows

package javaattacher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCommandWithBundledJar(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	defer os.Remove(f.Name())

	attacher, err := New(cfg)
	require.NoError(t, err)
	defer attacher.cleanResources()

	// invalid user and group ensures usage of the bundled jar
	jvm := &jvmDetails{
		pid:     12345,
		command: filepath.FromSlash("/home/someuser/java_home/bin/java"),
	}
	command := attacher.attachJVMCommand(context.Background(), jvm)
	want := filepath.FromSlash("/home/someuser/java_home/bin/java -jar java-attacher.jar") +
		" --log-level debug --config activation_method=FLEET --include-pid 12345 --download-agent-version 1.27.0 --config server_url=http://myhost:8200"

	cmdArgs := strings.Join(command.Args, " ")
	assert.Equal(t, want, cmdArgs)
	assert.Empty(t, attacher.tmpDirs)
	assert.Empty(t, attacher.uidToAttacherJar)
}
