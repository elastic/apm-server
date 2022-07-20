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
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/beater/config"

	"github.com/stretchr/testify/require"
)

func TestJavaBinaryDiscovery(t *testing.T) {
	cfg := config.JavaAttacherConfig{JavaBin: ""}
	t.Setenv("JAVA_HOME", "/usr/local")
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	attacher, _ := New(cfg)
	err = attacher.discoverJavaExecutable()
	require.NoError(t, err)
	javapath := filepath.FromSlash("/usr/local/bin/java")
	assert.Equal(t, javapath, attacher.javaBin)

	cfg.JavaBin = "/home/user/bin/java"
	attacher, _ = New(cfg)
	err = attacher.discoverJavaExecutable()
	require.NoError(t, err)
	javapath = filepath.FromSlash("/home/user/bin/java")
	assert.Equal(t, javapath, attacher.javaBin)
}

func TestBuildWithJvmDiscovery(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())

	attacher, err := New(cfg)
	require.NoError(t, err)

	jvm := &JvmDetails{
		pid:     "12345",
		command: "/home/someuser/java_home/bin/java",
	}
	cmd := attacher.build(context.Background(), jvm)
	want := filepath.FromSlash("/home/someuser/java_home/bin/java -jar ./java-attacher.jar") +
		" --log-level debug --include-pid 12345 " +
		"--download-agent-version 1.27.0 --config server_url=http://myhost:8200"

	cmdArgs := strings.Join(cmd.Args, " ")
	assert.Equal(t, want, cmdArgs)

	cfg.Config["service_name"] = "my-cool-service"
	attacher, err = New(cfg)
	require.NoError(t, err)

	cmd = attacher.build(context.Background(), jvm)
	cmdArgs = strings.Join(cmd.Args, " ")
	assert.Contains(t, cmdArgs, "--config server_url=http://myhost:8200")
	assert.Contains(t, cmdArgs, "--config service_name=my-cool-service")
}

func TestBuildWithoutJvmDiscovery(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())

	attacher, err := New(cfg)
	require.NoError(t, err)

	cmd := attacher.build(context.Background(), &JvmDetails{})
	want := filepath.FromSlash("/usr/bin/java -jar ./java-attacher.jar") +
		" --log-level debug --continuous --exclude-user root --include-main MyApplication " +
		"--include-main my-application.jar --include-vmarg elastic.apm.agent.attach=true " +
		"--download-agent-version 1.27.0 --config server_url=http://myhost:8200"

	cmdArgs := strings.Join(cmd.Args, " ")
	assert.Equal(t, want, cmdArgs)

	cfg.Config["service_name"] = "my-cool-service"
	attacher, err = New(cfg)
	require.NoError(t, err)

	cmd = attacher.build(context.Background(), &JvmDetails{})
	cmdArgs = strings.Join(cmd.Args, " ")
	assert.Contains(t, cmdArgs, "--config server_url=http://myhost:8200")
	assert.Contains(t, cmdArgs, "--config service_name=my-cool-service")
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
		JavaBin:              "/usr/bin/java",
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
		DiscoveryRules: args,
	}
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	javaAttacher, err := New(cfg)
	require.NoError(t, err)
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
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	javaAttacher, err := New(cfg)
	require.NoError(t, err)
	require.True(t, javaAttacher.enabled)
	require.Equal(t, "http://localhost:8200", javaAttacher.agentConfigs["server_url"])
	require.Equal(t, "1.25.0", javaAttacher.downloadAgentVersion)
	require.Len(t, javaAttacher.discoveryRules, 5)
	require.Equal(t, userDiscoveryRule{user: "root", isIncludeRule: false}, javaAttacher.discoveryRules[0])
	mainRegex, _ := regexp.Compile("MyApplication")
	require.Equal(t, cmdLineDiscoveryRule{argumentName: "include-main", regex: mainRegex, isIncludeRule: true}, javaAttacher.discoveryRules[1])
	require.Equal(t, userDiscoveryRule{user: "me", isIncludeRule: false}, javaAttacher.discoveryRules[2])
	vmargRegex, _ := regexp.Compile("-D.*attach=true")
	require.Equal(t, cmdLineDiscoveryRule{argumentName: "include-vmarg", regex: vmargRegex, isIncludeRule: true}, javaAttacher.discoveryRules[3])
	require.Equal(t, includeAllRule{}, javaAttacher.discoveryRules[4])

	jvmDetails := JvmDetails{
		user:      "me",
		uid:       "",
		gid:       "",
		pid:       "",
		startTime: "",
		command:   "",
		version:   "",
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
	require.IsType(t, userDiscoveryRule{}, match)
	require.Equal(t, "me", match.(userDiscoveryRule).user)
	require.False(t, match.include())
	javaAttacher.discoveryRules[2] = userDiscoveryRule{}
	match = javaAttacher.findFirstMatch(&jvmDetails)
	require.NotNil(t, match)
	require.IsType(t, cmdLineDiscoveryRule{}, match)
	require.Equal(t, vmargRegex, match.(cmdLineDiscoveryRule).regex)
	require.True(t, match.include())
	javaAttacher.discoveryRules[3] = userDiscoveryRule{}
	match = javaAttacher.findFirstMatch(&jvmDetails)
	require.NotNil(t, match)
	require.IsType(t, includeAllRule{}, match)
	require.True(t, match.include())
	javaAttacher.discoveryRules[4] = userDiscoveryRule{}
	require.Nil(t, javaAttacher.findFirstMatch(&jvmDetails))
}

func TestRunMode(t *testing.T) {
	cfg := createTestConfig()
	f, err := os.Create(javaAttacher)
	require.NoError(t, err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f.Name())
	attacher, err := New(cfg)
	require.NoError(t, err)
	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	attacher.Run(timeout)
	if runtime.GOOS == "windows" {
		require.NoError(t, timeout.Err())
	} else {
		err := timeout.Err()
		require.Error(t, err, "test")
		require.Equal(t, "context deadline exceeded", err.Error())
	}
}
