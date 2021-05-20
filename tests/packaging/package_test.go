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

type package_ struct {
	arch,
	path string
}

// TestDeb ensures debian packages are created and can be installed
func TestDeb(t *testing.T) {
	testInstall(t, "deb")
}

// TestRpm ensures rpm packages are created and can be installed
func TestRpm(t *testing.T) {
	testInstall(t, "rpm")
}

// (deb|rpm) would remove check that both types of packages are created
func testInstall(t *testing.T, ext string) {
	pkgs := getPackages(t, regexp.MustCompile(fmt.Sprintf(`-(\w+)\.%s$`, ext)))
	if len(pkgs) == 0 {
		t.Fatalf("no %ss found", ext)
	}
	for _, pkg := range pkgs {
		t.Run(fmt.Sprintf("%s_%s", t.Name(), pkg.arch), func(t *testing.T) {
			switch pkg.arch {
			case "amd64", "x86_64":
				checkInstall(t, pkg.path, fmt.Sprintf("Dockerfile.%s.%s.install", pkg.arch, ext))
			default:
				t.Skipf("skipped package install test for %s on %s", ext, pkg.arch)
			}
		})
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

func getPackages(t *testing.T, pattern *regexp.Regexp) []package_ {
	matches, err := filepath.Glob(*files)
	if err != nil {
		t.Fatal(err)
	}
	fs := make([]package_, 0)
	for _, f := range matches {
		if m := pattern.FindStringSubmatch(filepath.Base(f)); len(m) > 0 {
			fs = append(fs, package_{arch: m[1], path: f})
		}
	}

	return fs
}
