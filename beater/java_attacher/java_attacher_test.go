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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/config"
)

func TestNew(t *testing.T) {
	cfg := config.JavaAttacherConfig{JavaBin: ""}
	jh := os.Getenv("JAVA_HOME")
	os.Setenv("JAVA_HOME", "/usr/local")
	defer func() {
		// reset JAVA_HOME
		os.Setenv("JAVA_HOME", jh)
	}()

	attacher, err := New(cfg)
	require.NoError(t, err)

	javapath := filepath.FromSlash("/usr/local/bin/java")
	assert.Equal(t, javapath, attacher.cfg.JavaBin)

	cfg.JavaBin = "/home/user/bin/java"
	attacher, err = New(cfg)
	require.NoError(t, err)

	javapath = filepath.FromSlash("/home/user/bin/java")
	assert.Equal(t, javapath, attacher.cfg.JavaBin)
}

func TestBuild(t *testing.T) {
	args := []map[string]string{
		{"exclude-user": "root"},
		{"include-pid": "123"},
		{"include-pid": "456"},
		{"include-main": "MyApplication"},
		{"include-main": "my-application.jar"},
		{"include-vmarg": "elastic.apm.agent.attach=true"},
	}
	cfg := config.JavaAttacherConfig{
		Enabled:        true,
		DiscoveryRules: args,
		Config: map[string]string{
			"service_name": "my-cool-service",
			"server_url":   "http://localhost:8200",
		},
		JavaBin: "/usr/bin/java",
	}

	attacher, err := New(cfg)
	require.NoError(t, err)

	cmd := attacher.build(context.Background())

	want := "/usr/bin/java -jar /bin/apm-agent-attach-cli-1.24.0-slim.jar --continuous " +
		"--exclude-user root --include-pid 123 --include-pid 456 " +
		"--include-main MyApplication --include-main my-application.jar --include-vmarg elastic.apm.agent.attach=true " +
		"--config service_name=my-cool-service --config server_url=http://localhost:8200"

	cmdArgs := strings.Join(cmd.Args, " ")
	assert.Equal(t, want, cmdArgs)
}
