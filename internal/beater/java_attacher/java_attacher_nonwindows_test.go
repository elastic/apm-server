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

//go:build !windows
// +build !windows

package javaattacher

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInvalidCommand(t *testing.T) {
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
		uid:     "invalid",
		command: filepath.FromSlash("/home/someuser/java_home/bin/java"),
	}
	command := attacher.attachJVMCommand(context.Background(), jvm)
	assert.Nil(t, command)
	assert.Empty(t, attacher.tmpDirs)
	assert.Len(t, attacher.uidToAttacherJar, 1)
}

func TestBuildCommandWithTempJar(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	defer os.Remove(f.Name())

	attacher, err := New(cfg)
	require.NoError(t, err)
	defer attacher.cleanResources()

	currentUser, _ := user.Current()
	jvm := &jvmDetails{
		pid:     12345,
		uid:     currentUser.Uid,
		gid:     currentUser.Gid,
		command: filepath.FromSlash("/home/someuser/java_home/bin/java"),
	}
	assert.Empty(t, attacher.tmpDirs)
	assert.Empty(t, attacher.uidToAttacherJar)
	command := attacher.attachJVMCommand(context.Background(), jvm)
	assert.Len(t, attacher.tmpDirs, 1)
	assert.Len(t, attacher.uidToAttacherJar, 1)
	attacherJar := attacher.uidToAttacherJar[currentUser.Uid]
	assert.NotEqual(t, attacherJar, "")
	require.NotEqual(t, bundledJavaAttacher, attacherJar)
	want := filepath.FromSlash(fmt.Sprintf("/home/someuser/java_home/bin/java -jar %v", attacherJar)) +
		" --log-level debug --config activation_method=FLEET --include-pid 12345 --download-agent-version 1.27.0 --config server_url=http://myhost:8200"

	cmdArgs := strings.Join(command.Args, " ")
	assert.Equal(t, want, cmdArgs)

	cfg.Config["service_name"] = "my-cool-service"
	attacher, err = New(cfg)
	require.NoError(t, err)
	defer attacher.cleanResources()

	command = attacher.attachJVMCommand(context.Background(), jvm)
	cmdArgs = strings.Join(command.Args, " ")
	assert.Contains(t, cmdArgs, "--config server_url=http://myhost:8200")
	assert.Contains(t, cmdArgs, "--config service_name=my-cool-service")
	assert.Contains(t, cmdArgs, "--config activation_method=FLEET")
}

func TestTempDirCreation(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	defer os.Remove(f.Name())
	attacher, err := New(cfg)
	require.NoError(t, err)
	defer attacher.cleanResources()

	currentUser, _ := user.Current()
	jvm := &jvmDetails{
		pid:     12345,
		uid:     currentUser.Uid,
		gid:     currentUser.Gid,
		command: filepath.FromSlash("/home/someuser/java_home/bin/java"),
	}
	assert.Empty(t, attacher.tmpDirs)
	assert.Empty(t, attacher.uidToAttacherJar)
	// this call creates the temp dir
	attacher.attachJVMCommand(context.Background(), jvm)
	assert.Len(t, attacher.uidToAttacherJar, 1)
	attacherJar := attacher.uidToAttacherJar[currentUser.Uid]
	assert.NotEqual(t, attacherJar, "")
	require.NotEqual(t, bundledJavaAttacher, attacherJar)

	assert.Len(t, attacher.tmpDirs, 1)
	tempAttacherDir := attacher.tmpDirs[0]
	require.DirExists(t, tempAttacherDir)
	attacherDirInfo, err := os.Stat(tempAttacherDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), attacherDirInfo.Mode().Perm())
	tempDir, err := os.Open(tempAttacherDir)
	require.NoError(t, err)
	files, err := tempDir.ReadDir(0)
	require.NoError(t, err)
	require.Len(t, files, 1)
	attacherJarFileInfo, err := files[0].Info()
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), attacherJarFileInfo.Mode().Perm())
	require.Equal(t, attacherJar, filepath.Join(tempAttacherDir, attacherJarFileInfo.Name()))

	// verify caching
	attacher.attachJVMCommand(context.Background(), jvm)
	assert.Len(t, attacher.tmpDirs, 1)
	assert.Len(t, attacher.uidToAttacherJar, 1)
}
