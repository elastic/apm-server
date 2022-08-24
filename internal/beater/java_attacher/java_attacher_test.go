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

package javaattacher

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
)

func TestNoAttacherCreatedWithoutDiscoveryRules(t *testing.T) {
	cfg := config.JavaAttacherConfig{
		Enabled: true,
	}
	_, err := New(cfg)
	require.Error(t, err)
}

func TestBuildCommandWithBundledJar(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())

	attacher, err := New(cfg)
	require.NoError(t, err)
	defer attacher.cleanResources()

	// invalid user and group ensures usage of the bundled jar
	jvm := &jvmDetails{
		pid:     12345,
		uid:     "invalid",
		gid:     "invalid",
		command: filepath.FromSlash("/home/someuser/java_home/bin/java"),
	}
	command := attacher.attachJVMCommand(context.Background(), jvm)
	want := filepath.FromSlash("/home/someuser/java_home/bin/java -jar java-attacher.jar") +
		" --log-level debug --include-pid 12345 --download-agent-version 1.27.0 --config server_url=http://myhost:8200"

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
}

func TestBuildCommandWithTempJar(t *testing.T) {
	// todo: what's the proper way to exclude tests for a specific OS?
	if runtime.GOOS == "windows" {
		return
	}
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
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
		" --log-level debug --include-pid 12345 --download-agent-version 1.27.0 --config server_url=http://myhost:8200"

	cmdArgs := strings.Join(command.Args, " ")
	assert.Equal(t, want, cmdArgs)
}

func createTestConfig() config.JavaAttacherConfig {
	args := []map[string]string{
		{"exclude-user": "root"},
		{"include-main": "MyApplication"},
		{"include-main": "my-application.jar"},
		{"include-vmarg": "elastic.apm.agent.attach=true"},
	}
	cfg := config.JavaAttacherConfig{
		Enabled:        true,
		DiscoveryRules: args,
		Config: map[string]string{
			"server_url": "http://myhost:8200",
		},
		DownloadAgentVersion: "1.27.0",
	}
	return cfg
}

func TestDiscoveryRulesAllowlist(t *testing.T) {
	allowlistLength := len(config.JavaAttacherAllowlist)
	args := make([]map[string]string, allowlistLength+1)
	for discoveryRuleKey := range config.JavaAttacherAllowlist {
		args = append(args, map[string]string{discoveryRuleKey: "test"})
	}
	args = append(args, map[string]string{"invalid": "test"})
	cfg := config.JavaAttacherConfig{
		Enabled:        true,
		DiscoveryRules: args,
	}
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	javaAttacher, err := New(cfg)
	require.NoError(t, err)
	defer javaAttacher.cleanResources()
	discoveryRules := javaAttacher.discoveryRules
	require.Len(t, discoveryRules, allowlistLength)
}

func TestConfig(t *testing.T) {
	args := []map[string]string{
		{"exclude-user": "root"},
		{"include-main": "MyApplication"},
		{"exclude-user": "me"},
		{"include-vmarg": "-D.*attach=true"},
		{"include-all": "ignored"},
	}
	cfg := config.JavaAttacherConfig{
		Enabled:        true,
		DiscoveryRules: args,
		Config: map[string]string{
			"server_url": "http://localhost:8200",
		},
		DownloadAgentVersion: "1.25.0",
	}
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	javaAttacher, err := New(cfg)
	require.NoError(t, err)
	defer javaAttacher.cleanResources()
	require.True(t, javaAttacher.enabled)
	require.Equal(t, "http://localhost:8200", javaAttacher.agentConfigs["server_url"])
	require.Equal(t, "1.25.0", javaAttacher.downloadAgentVersion)
	require.Len(t, javaAttacher.discoveryRules, 5)
	require.Equal(t, &userDiscoveryRule{user: "root", isIncludeRule: false}, javaAttacher.discoveryRules[0])
	mainRegex, _ := regexp.Compile("MyApplication")
	require.Equal(t, &cmdLineDiscoveryRule{argumentName: "include-main", regex: mainRegex, isIncludeRule: true}, javaAttacher.discoveryRules[1])
	require.Equal(t, &userDiscoveryRule{user: "me", isIncludeRule: false}, javaAttacher.discoveryRules[2])
	vmargRegex, _ := regexp.Compile("-D.*attach=true")
	require.Equal(t, &cmdLineDiscoveryRule{argumentName: "include-vmarg", regex: vmargRegex, isIncludeRule: true}, javaAttacher.discoveryRules[3])
	require.Equal(t, includeAllRule{}, javaAttacher.discoveryRules[4])

	jvmDetails := jvmDetails{
		user:    "me",
		uid:     "",
		gid:     "",
		command: "",
		version: "",
		cmdLineArgs: "org.apache.catalina.startup.Bootstrap --add-opens=java.base/java.lang=ALL-UNNAMED " +
			"--add-opens=java.base/java.io=ALL-UNNAMED --add-opens=java.base/java.util=ALL-UNNAMED " +
			"--add-opens=java.base/java.util.concurrent=ALL-UNNAMED " +
			"--add-opens=java.rmi/sun.rmi.transport=ALL-UNNAMED " +
			"-Djava.util.logging.config.file=/Users/eyalkoren/tests/apache-tomcat-9.0.58/conf/logging.properties " +
			"-Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager -Djdk.tls.ephemeralDHKeySize=2048 " +
			"-Djava.protocol.handler.pkgs=org.apache.catalina.webresources -Dorg.apache.catalina.security.SecurityListener.UMASK=0027 " +
			"-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005 -Delastic.apm.service_name=Tomcat9 " +
			"-Dignore.endorsed.dirs= -Dcatalina.base=/Users/eyalkoren/tests/apache-tomcat-9.0.58 " +
			"-Delastic.apm.agent.attach=true " +
			"-Dcatalina.home=/Users/eyalkoren/tests/apache-tomcat-9.0.58 -Djava.io.tmpdir=/Users/eyalkoren/tests/apache-tomcat-9.0.58/temp",
	}

	match := javaAttacher.findFirstMatch(&jvmDetails)
	require.NotNil(t, match)
	require.IsType(t, &userDiscoveryRule{}, match)
	require.Equal(t, "me", match.(*userDiscoveryRule).user)
	require.False(t, match.include())
	javaAttacher.discoveryRules[2] = &userDiscoveryRule{}
	match = javaAttacher.findFirstMatch(&jvmDetails)
	require.NotNil(t, match)
	require.IsType(t, &cmdLineDiscoveryRule{}, match)
	require.Equal(t, vmargRegex, match.(*cmdLineDiscoveryRule).regex)
	require.True(t, match.include())
	javaAttacher.discoveryRules[3] = &userDiscoveryRule{}
	match = javaAttacher.findFirstMatch(&jvmDetails)
	require.NotNil(t, match)
	require.IsType(t, includeAllRule{}, match)
	require.True(t, match.include())
	javaAttacher.discoveryRules[4] = &userDiscoveryRule{}
	require.Nil(t, javaAttacher.findFirstMatch(&jvmDetails))
}

func TestTempDirCreation(t *testing.T) {
	// todo: what's the proper way to exclude tests for a specific OS?
	if runtime.GOOS == "windows" {
		return
	}
	cfg := createTestConfig()
	f, err := os.Create(bundledJavaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
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
