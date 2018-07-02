// +build mage

package main

import (
	"fmt"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"github.com/elastic/beats/dev-tools/mage"
)

func init() {
	mage.SetElasticBeatsDir("./_beats")

	mage.SetBuildVariableSources(&mage.BuildVariableSources{
		BeatVersion: "vendor/github.com/elastic/beats/libbeat/version/version.go",
		GoVersion:   ".go-version",
		DocBranch:   "docs/version.asciidoc",
	})

	mage.BeatDescription = "Elastic APM Server"
	mage.BeatURL         = "https://www.elastic.co/solutions/apm"
	mage.BeatIndexPrefix = "apm"
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

// CrossBuildGoDaemon cross-builds the go-daemon binary using Docker.
func CrossBuildGoDaemon() error {
	return mage.CrossBuildGoDaemon()
}

// Clean cleans all generated files and build artifacts.
func Clean() error {
	return mage.Clean()
}

// Package packages the Beat for distribution.
// Use SNAPSHOT=true to build snapshots.
// Use PLATFORMS to control the target platforms.
func Package() {
	start := time.Now()
	defer func() { fmt.Println("package ran for", time.Since(start)) }()

	mage.UseElasticBeatPackaging()
	customizePackaging()

	mg.Deps(Update)
	mg.Deps(CrossBuild, CrossBuildGoDaemon)
	mg.SerialDeps(mage.Package, TestPackages)
}

// TestPackages tests the generated packages (i.e. file modes, owners, groups).
func TestPackages() error {
	return mage.TestPackages()
}

// Update updates the generated files (aka make update).
func Update() error {
	return sh.Run("make", "update")
}

// -----------------------------------------------------------------------------
// Customizations specific to apm-server.
// - readme.md.tmpl used in packages is customized.
// - apm-server.reference.yml is not included in packages.

func customizePackaging() {
	var (
		readmeTemplate = mage.PackageFile{
			Mode: 0644,
			Template: "package-README.md.tmpl",
		}
	)
	for _, args := range mage.Packages {
		switch args.Types[0] {
		case mage.Zip, mage.TarGz:
			// Remove the reference config file from packages.
			delete(args.Spec.Files, "{{.BeatName}}.reference.yml")

			// Replace the README.md with an APM specific file.
			args.Spec.ReplaceFile("README.md", readmeTemplate)
		case mage.Deb, mage.RPM:
			delete(args.Spec.Files, "/etc/{{.BeatName}}/{{.BeatName}}.reference.yml")
			args.Spec.ReplaceFile("/usr/share/{{.BeatName}}/README.md", readmeTemplate)
		}
	}
}
