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

//go:build mage
// +build mage

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/dev-tools/mage"

	"github.com/elastic/apm-server/internal/beater/config"
)

var versionFileRegex = regexp.MustCompile(`(?m)^const Version = "(.+)"\r?$`)

func init() {
	repo, err := mage.GetProjectRepoInfo()
	if err != nil {
		panic(err)
	}
	mage.SetBuildVariableSources(&mage.BuildVariableSources{
		BeatVersion: filepath.Join(repo.RootDir, "internal", "version", "version.go"),
		BeatVersionParser: func(data []byte) (string, error) {
			matches := versionFileRegex.FindSubmatch(data)
			if len(matches) == 2 {
				return string(matches[1]), nil
			}
			return "", errors.New("failed to parse version file")

		},
		GoVersion: filepath.Join(repo.RootDir, ".go-version"),
		DocBranch: filepath.Join(repo.RootDir, "docs/version.asciidoc"),
	})

	// Filter platforms to those that are supported by apm-server.
	mage.Platforms = mage.Platforms.Filter(strings.Join([]string{
		"linux/amd64",
		"linux/386",
		"linux/arm64",
		"windows/386",
		"windows/amd64",
		"darwin/amd64",
	}, " "))

	mage.BeatDescription = "Elastic APM Server"
	mage.BeatURL = "https://www.elastic.co/apm"
	mage.BeatIndexPrefix = "apm"
	mage.XPackDir = "x-pack"
	mage.BeatUser = "apm-server"
}

// Build builds the Beat binary.
func Build() error {
	args := mage.DefaultBuildArgs()
	args.InputFiles = []string{"./x-pack/apm-server"}
	args.Name += "-" + mage.Platform.GOOS + "-" + mage.Platform.Arch
	args.OutputDir = "build"
	args.CGO = false
	if mage.Platform.Arch == "386" {
		// Only enable PIE on 64-bit platforms.
		args.BuildMode = ""
	}
	return mage.Build(args)
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

// Ironbank packages apm-server for the Ironbank distribution, relying on the
// binaries having already been built.
//
// Use SNAPSHOT=true to build snapshots.
func Ironbank() error {
	if runtime.GOARCH != "amd64" {
		fmt.Printf(">> Ironbank images are only supported for amd64 arch (%s is not supported)\n", runtime.GOARCH)
		return nil
	}
	if err := prepareIronbankBuild(); err != nil {
		return errors.Wrap(err, "failed to prepare the ironbank context")
	}
	if err := saveIronbank(); err != nil {
		return errors.Wrap(err, "failed to save artifacts for ironbank")
	}
	return nil
}

// Package packages apm-server for distribution, relying on the
// binaries having already been built.
//
// Use SNAPSHOT=true to build snapshots.
// Use PLATFORMS to control the target platforms. eg linux/amd64
// Use TYPES to control the target types. eg docker
func Package() error {
	mage.UseElasticBeatXPackPackaging()
	customizePackaging()
	if packageTypes := os.Getenv("TYPES"); packageTypes != "" {
		filterPackages(packageTypes)
	}
	return mage.Package()
}

// -----------------------------------------------------------------------------

func customizePackaging() {
	const emptyDir = "build/empty"
	if err := os.MkdirAll(emptyDir, 0750); err != nil {
		panic(errors.Wrapf(err, "failed to create dir %v", emptyDir))
	}

	for idx := len(mage.Packages) - 1; idx >= 0; idx-- {
		args := &mage.Packages[idx]

		// Replace "build/golang-crossbuild" with "build" in the sources.
		trimCrossbuildPrefix := filepath.Join("build", "golang-cross")
		for filename, filespec := range args.Spec.Files {
			filespec.Source = strings.TrimPrefix(filespec.Source, trimCrossbuildPrefix)
			args.Spec.Files[filename] = filespec
		}

		// Replace the generic Beats README.md with an APM specific one, and remove files unused by apm-server.
		for filename, filespec := range args.Spec.Files {
			switch filespec.Source {
			case "{{ elastic_beats_dir }}/dev-tools/packaging/templates/common/README.md.tmpl":
				args.Spec.Files[filename] = mage.PackageFile{Mode: 0644, Template: "packaging/files/README.md.tmpl"}
			case "_meta/kibana.generated", "fields.yml", "{{.BeatName}}.reference.yml":
				delete(args.Spec.Files, filename)
			}
		}

		switch pkgType := args.Types[0]; pkgType {
		case mage.Zip, mage.TarGz:
			args.Spec.Files["java-attacher.jar"] = mage.PackageFile{Mode: 0755, Source: "build/java-attacher.jar", Owner: mage.BeatUser}

		case mage.Docker:
			args.Spec.ExtraVars["expose_ports"] = config.DefaultPort
			args.Spec.ExtraVars["repository"] = "docker.elastic.co/apm"
			args.Spec.Files["java-attacher.jar"] = mage.PackageFile{Mode: 0755, Source: "build/java-attacher.jar", Owner: mage.BeatUser}

		case mage.Deb, mage.RPM:
			// Update config file owner.
			pf := args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"]
			pf.Owner = mage.BeatUser
			args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"] = pf
			args.Spec.Files["/var/log/{{.BeatName}}"] = mage.PackageFile{Mode: 0750, Source: emptyDir, Owner: mage.BeatUser}
			args.Spec.Files["/usr/share/{{.BeatName}}/bin/java-attacher.jar"] = mage.PackageFile{Mode: 0755,
				Source: "build/java-attacher.jar", Owner: mage.BeatUser}

			// Customise the pre-install and post-install scripts.
			args.Spec.PreInstallScript = "packaging/files/linux/pre-install.sh.tmpl"
			if pkgType == mage.Deb {
				args.Spec.PostInstallScript = "packaging/files/linux/deb-post-install.sh.tmpl"
			}

			// All our supported Linux distros have systemd, so don't package any SystemV init scripts or go-daemon.
			delete(args.Spec.Files, "/usr/share/{{.BeatName}}/bin/{{.BeatName}}-god")
			delete(args.Spec.Files, "/etc/init.d/{{.BeatServiceName}}")

		default:
			panic(errors.Errorf("unhandled package type: %v", pkgType))
		}
	}
}

func saveIronbank() error {
	fmt.Println(">> saveIronbank: save the IronBank container context.")

	ironbank := getIronbankContextName()
	buildDir := filepath.Join("build", ironbank)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return fmt.Errorf("cannot find the folder with the ironbank context: %+v", err)
	}

	distributionsDir := "build/distributions"
	if _, err := os.Stat(distributionsDir); os.IsNotExist(err) {
		err := os.MkdirAll(distributionsDir, 0750)
		if err != nil {
			return fmt.Errorf("cannot create folder for docker artifacts: %+v", err)
		}
	}

	// change dir to the buildDir location where the ironbank folder exists
	// this will generate a tar.gz without some nested folders.
	wd, _ := os.Getwd()
	os.Chdir(buildDir)
	defer os.Chdir(wd)

	// move the folder to the parent folder, there are two parent folder since
	// buildDir contains a two folders dir.
	tarGzFile := filepath.Join("..", "..", distributionsDir, ironbank+".tar.gz")

	// Save the build context as tar.gz artifact
	err := mage.Tar("./", tarGzFile)
	if err != nil {
		return fmt.Errorf("cannot compress the tar.gz file: %+v", err)
	}

	return errors.Wrap(mage.CreateSHA512File(tarGzFile), "failed to create .sha512 file")
}

func getIronbankContextName() string {
	version, _ := mage.BeatQualifiedVersion()
	defaultBinaryName := "{{.Name}}-ironbank-{{.Version}}{{if .Snapshot}}-SNAPSHOT{{end}}"
	outputDir, _ := mage.Expand(defaultBinaryName+"-docker-build-context", map[string]interface{}{
		"Name":    "apm-server",
		"Version": version,
	})
	return outputDir
}

func prepareIronbankBuild() error {
	fmt.Println(">> prepareIronbankBuild: prepare the IronBank container context.")
	ironbank := getIronbankContextName()
	buildDir := filepath.Join("build", ironbank)
	templatesDir := filepath.Join("packaging", "ironbank")

	data := map[string]interface{}{
		"MajorMinor": majorMinor(),
	}

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			target := strings.TrimSuffix(
				filepath.Join(buildDir, filepath.Base(path)),
				".tmpl",
			)

			err := mage.ExpandFile(path, target, data)
			if err != nil {
				return errors.Wrapf(err, "expanding template '%s' to '%s'", path, target)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func majorMinor() string {
	if v, _ := mage.BeatQualifiedVersion(); v != "" {
		parts := strings.SplitN(v, ".", 3)
		return parts[0] + "." + parts[1]
	}
	return ""
}
