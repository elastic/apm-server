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

// +build package

package test

import (
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/magefile/mage/sh"
)

var (
	files = flag.String("files", "build/distributions/*", "filepath glob containing package files")
)

func TestDeb(t *testing.T) {
	debs := getFiles(t, regexp.MustCompile(`\.deb$`))
	if len(debs) == 0 {
		t.Fatal("no debs found")
	}
	for _, deb := range debs {
		checkInstall(t, deb, "Dockerfile.deb.install")
	}
}

func TestRpm(t *testing.T) {
	rpms := getFiles(t, regexp.MustCompile(`\.rpm$`))
	if len(rpms) == 0 {
		t.Fatal("no rpms found")
	}
	for _, rpm := range rpms {
		checkInstall(t, rpm, "Dockerfile.rpm.install")
	}
}

func checkInstall(t *testing.T, pkg, dockerfile string) {
	dir, file := filepath.Split(pkg)
	imageId, err := sh.Output(
		"docker", "build", "--no-cache", "-q", "-f", dockerfile,
		"--build-arg", fmt.Sprintf("apm_server_pkg=%s", file), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.Run("docker", "run", "--rm", imageId); err != nil {
		t.Fatal(err)
	}
}

func getFiles(t *testing.T, pattern *regexp.Regexp) []string {
	matches, err := filepath.Glob(*files)
	if err != nil {
		t.Fatal(err)
	}
	fs := matches[:0]
	for _, f := range matches {
		if pattern.MatchString(filepath.Base(f)) {
			fs = append(fs, f)
		}
	}

	return fs
}
