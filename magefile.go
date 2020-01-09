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

// +build mage

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/pkg/errors"

	"github.com/elastic/beats/dev-tools/mage"

	"github.com/elastic/apm-server/beater/config"
)

func init() {

	mage.SetBuildVariableSources(&mage.BuildVariableSources{
		BeatVersion: "vendor/github.com/elastic/beats/libbeat/version/version.go",
		GoVersion:   ".go-version",
		DocBranch:   "docs/version.asciidoc",
	})

	mage.BeatDescription = "Elastic APM Server"
	mage.BeatURL = "https://www.elastic.co/products/apm"
	mage.BeatIndexPrefix = "apm"
	mage.XPackDir = "x-pack"
	mage.BeatUser = "apm-server"
}

// Build builds the Beat binary.
func Build() error {
	return mage.Build(mage.DefaultBuildArgs())
}

// GolangCrossBuild build the Beat binary inside of the golang-builder.
// Do not use directly, use crossBuild instead.
func GolangCrossBuild() error {
	return mage.GolangCrossBuild(mage.DefaultGolangCrossBuildArgs())
}

// BuildGoDaemon builds the go-daemon binary (use crossBuildGoDaemon).
func BuildGoDaemon() error {
	return mage.BuildGoDaemon()
}

// CrossBuild cross-builds the beat for all target platforms.
func CrossBuild() error {
	return mage.CrossBuild()
}

func CrossBuildXPack() error {
	return mage.CrossBuildXPack()
}

// CrossBuildGoDaemon cross-builds the go-daemon binary using Docker.
func CrossBuildGoDaemon() error {
	return mage.CrossBuildGoDaemon()
}

// Clean cleans all generated files and build artifacts.
func Clean() error {
	return mage.Clean()
}

func Config() error {
	if err := mage.Config(mage.ShortConfigType, shortConfigFileParams(), "."); err != nil {
		return err
	}
	return mage.Config(mage.DockerConfigType, dockerConfigFileParams(), ".")
}

func shortConfigFileParams() mage.ConfigFileParams {
	return mage.ConfigFileParams{
		ShortParts: []string{
			mage.OSSBeatDir("_meta/beat.yml"),
		},
		ExtraVars: map[string]interface{}{
			"elasticsearch_hostport": "localhost:9200",
			"listen_hostport":        "localhost:" + config.DefaultPort,
			"jaeger_grpc_hostport":   "localhost:14250",
			"jaeger_http_hostport":   "localhost:14268",
		},
	}
}

func dockerConfigFileParams() mage.ConfigFileParams {
	return mage.ConfigFileParams{
		DockerParts: []string{
			mage.OSSBeatDir("_meta/beat.yml"),
		},
		ExtraVars: map[string]interface{}{
			"elasticsearch_hostport": "elasticsearch:9200",
			"listen_hostport":        "0.0.0.0:" + config.DefaultPort,
			"jaeger_grpc_hostport":   "0.0.0.0:14250",
			"jaeger_http_hostport":   "0.0.0.0:14268",
		},
	}
}

func keepPackages(types []string) map[mage.PackageType]struct{} {
	keep := make(map[mage.PackageType]struct{})
	for _, t := range types {
		var pt mage.PackageType
		if err := pt.UnmarshalText([]byte(t)); err != nil {
			log.Printf("skipped filtering package type %s", t)
			continue
		}
		keep[pt] = struct{}{}
	}
	return keep
}

func filterPackages(types string) {
	var packages []mage.OSPackageArgs
	keep := keepPackages(strings.Split(types, " "))
	for _, p := range mage.Packages {
		for _, t := range p.Types {
			if _, ok := keep[t]; !ok {
				continue
			}
			packages = append(packages, p)
			break
		}
	}
	mage.Packages = packages
}

// Package packages the Beat for distribution.
// Use SNAPSHOT=true to build snapshots.
// Use PLATFORMS to control the target platforms. eg linux/amd64
// Use TYPES to control the target types. eg docker
func Package() {
	start := time.Now()
	defer func() { fmt.Println("package ran for", time.Since(start)) }()

	mage.UseElasticBeatPackaging()
	customizePackaging()

	if packageTypes := os.Getenv("TYPES"); packageTypes != "" {
		filterPackages(packageTypes)
	}

	if os.Getenv("SKIP_BUILD") != "true" {
		mg.Deps(Update, prepareIngestPackaging)
		mg.Deps(CrossBuild, CrossBuildXPack, CrossBuildGoDaemon)
	}
	mg.SerialDeps(mage.Package, TestPackages)
}

// TestPackages tests the generated packages (i.e. file modes, owners, groups).
func TestPackages() error {
	return mage.TestPackages()
}

// TestPackagesInstall integration tests the generated packages
func TestPackagesInstall() error {
	// make the test script available to containers first
	copy := &mage.CopyTask{
		Source: "tests/packaging/test.sh",
		Dest:   mage.MustExpand("{{.PWD}}/build/distributions/test.sh"),
		Mode:   0755,
	}
	if err := copy.Execute(); err != nil {
		return err
	}
	defer sh.Rm(copy.Dest)

	goTest := sh.OutCmd("go", "test")
	var args []string
	if mg.Verbose() {
		args = append(args, "-v")
	}
	args = append(args, mage.MustExpand("tests/packaging/package_test.go"))
	args = append(args, "-timeout", "20m")
	args = append(args, "-files", mage.MustExpand("{{.PWD}}/build/distributions/*"))
	args = append(args, "-tags=package")

	if out, err := goTest(args...); err != nil {
		if mg.Verbose() {
			fmt.Println(out)
		}
		return err
	}

	return nil
}

// Update updates the generated files (aka make update).
func Update() error {
	return sh.Run("make", "update")
}

func Fields() error {
	return mage.GenerateFieldsYAML("model")
}

// Use RACE_DETECTOR=true to enable the race detector.
func GoTestUnit(ctx context.Context) error {
	return mage.GoTest(ctx, mage.DefaultGoTestUnitArgs())
}

// GoTestIntegration executes the Go integration tests.
// Use TEST_COVERAGE=true to enable code coverage profiling.
// Use RACE_DETECTOR=true to enable the race detector.
func GoTestIntegration(ctx context.Context) error {
	return mage.GoTest(ctx, mage.DefaultGoTestIntegrationArgs())
}

// -----------------------------------------------------------------------------

// Customizations specific to apm-server.
// - readme.md.tmpl used in packages is customized.
// - apm-server.reference.yml is not included in packages.
// - ingest .json files are included in packaging

var emptyDir = filepath.Clean("build/empty")
var ingestDirGenerated = filepath.Clean("build/packaging/ingest")

func customizePackaging() {
	if err := os.MkdirAll(emptyDir, 0750); err != nil {
		panic(errors.Wrapf(err, "failed to create dir %v", emptyDir))
	}

	var (
		readmeTemplate = mage.PackageFile{
			Mode:     0644,
			Template: "packaging/files/README.md.tmpl",
		}
		ingestTarget = "ingest"
		ingest       = mage.PackageFile{
			Mode:   0644,
			Source: ingestDirGenerated,
		}
	)
	for idx := len(mage.Packages) - 1; idx >= 0; idx-- {
		args := &mage.Packages[idx]
		pkgType := args.Types[0]
		switch pkgType {

		case mage.Zip, mage.TarGz:
			// Remove the reference config file from packages.
			delete(args.Spec.Files, "{{.BeatName}}.reference.yml")

			// Replace the README.md with an APM specific file.
			args.Spec.ReplaceFile("README.md", readmeTemplate)
			args.Spec.Files[ingestTarget] = ingest

		case mage.Docker:
			delete(args.Spec.Files, "{{.BeatName}}.reference.yml")
			args.Spec.ReplaceFile("README.md", readmeTemplate)
			args.Spec.Files[ingestTarget] = ingest
			args.Spec.ExtraVars["expose_ports"] = config.DefaultPort
			args.Spec.ExtraVars["repository"] = "docker.elastic.co/apm"

		case mage.Deb, mage.RPM:
			delete(args.Spec.Files, "/etc/{{.BeatName}}/{{.BeatName}}.reference.yml")
			args.Spec.ReplaceFile("/usr/share/{{.BeatName}}/README.md", readmeTemplate)
			args.Spec.Files["/usr/share/{{.BeatName}}/"+ingestTarget] = ingest

			// update config file Owner
			pf := args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"]
			pf.Owner = mage.BeatUser
			args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"] = pf

			args.Spec.Files["/var/lib/{{.BeatName}}"] = mage.PackageFile{Mode: 0750, Source: emptyDir, Owner: mage.BeatUser}
			args.Spec.Files["/var/log/{{.BeatName}}"] = mage.PackageFile{Mode: 0750, Source: emptyDir, Owner: mage.BeatUser}
			args.Spec.PreInstallScript = "packaging/files/linux/pre-install.sh.tmpl"
			if pkgType == mage.Deb {
				args.Spec.PostInstallScript = "packaging/files/linux/deb-post-install.sh.tmpl"
			}

		case mage.DMG:
			mage.Packages = append(mage.Packages[:idx], mage.Packages[idx+1:]...)

		default:
			panic(errors.Errorf("unhandled package type: %v", pkgType))

		}
	}
}

func prepareIngestPackaging() error {
	if err := sh.Rm(ingestDirGenerated); err != nil {
		return err
	}

	copy := &mage.CopyTask{
		Source:  "ingest",
		Dest:    ingestDirGenerated,
		Mode:    0644,
		DirMode: 0755,
		Exclude: []string{".*.go"},
	}
	return copy.Execute()

}

// DumpVariables writes the template variables and values to stdout.
func DumpVariables() error {
	return mage.DumpVariables()
}

func Check() error {
	fmt.Println(">> check: Checking source code for common problems")

	mg.Deps(mage.GoVet, mage.CheckNosetestsNotExecutable, mage.CheckYAMLNotExecutable)

	changes, err := mage.GitDiffIndex()
	if err != nil {
		return errors.Wrap(err, "failed to diff the git index")
	}
	if len(changes) > 0 {
		if mg.Verbose() {
			mage.GitDiff()
		}
		return errors.Errorf("some files are not up-to-date. "+
			"Run 'mage fmt update' then review and commit the changes. "+
			"Modified: %v", changes)
	}
	return nil
}

func Fmt() {
	mg.Deps(mage.Format)
}
