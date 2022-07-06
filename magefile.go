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
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/dev-tools/mage"

	"github.com/elastic/apm-server/beater/config"
)

func init() {
	repo, err := mage.GetProjectRepoInfo()
	if err != nil {
		panic(err)
	}
	mage.SetBuildVariableSources(&mage.BuildVariableSources{
		BeatVersion: filepath.Join(repo.RootDir, "cmd", "version.go"),
		GoVersion:   filepath.Join(repo.RootDir, ".go-version"),
		DocBranch:   filepath.Join(repo.RootDir, "docs/version.asciidoc"),
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
	mage.VirtualenvReqs = []string{filepath.Join(repo.RootDir, "script", "requirements.txt")}
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

// Clean cleans all generated files and build artifacts.
func Clean() error {
	return mage.Clean()
}

// Config generates apm-server.yml and apm-server.docker.yml.
func Config() error {
	if err := mage.Config(mage.ShortConfigType, shortConfigFileParams(), "."); err != nil {
		return err
	}
	return mage.Config(mage.DockerConfigType, dockerConfigFileParams(), ".")
}

func shortConfigFileParams() mage.ConfigFileParams {
	return mage.ConfigFileParams{
		Short: mage.ConfigParams{Template: mage.OSSBeatDir("_meta/beat.yml")},
		ExtraVars: map[string]interface{}{
			"elasticsearch_hostport": "localhost:9200",
			"listen_hostport":        "localhost:" + config.DefaultPort,
		},
	}
}

func dockerConfigFileParams() mage.ConfigFileParams {
	return mage.ConfigFileParams{
		Docker: mage.ConfigParams{Template: mage.OSSBeatDir("_meta/beat.yml")},
		ExtraVars: map[string]interface{}{
			"elasticsearch_hostport": "elasticsearch:9200",
			"listen_hostport":        "0.0.0.0:" + config.DefaultPort,
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

// Package packages apm-server for the IronBank distribution, relying on the
// binaries having already been built.
//
// Use SNAPSHOT=true to build snapshots.
func Ironbank() error {
	if runtime.GOARCH != "amd64" {
		fmt.Printf(">> IronBank images are only supported for amd64 arch (%s is not supported)\n", runtime.GOARCH)
		return nil
	}
	if err := prepareIronbankBuild(); err != nil {
		return errors.Wrap(err, "failed to prepare build")
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

// Version prints out the qualified stack version.
func Version() error {
	v, err := mage.BeatQualifiedVersion()
	if err != nil {
		return err
	}
	fmt.Print(v)
	return nil
}

// Update updates the generated files.
func Update() error {
	mg.Deps(Config)
	return nil
}

// Use RACE_DETECTOR=true to enable the race detector.
func GoTestUnit(ctx context.Context) error {
	return mage.GoTest(ctx, mage.DefaultGoTestUnitArgs())
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
			args.Spec.Files["java-attacher.jar"] = mage.PackageFile{Mode: 0750, Source: "build/java-attacher.jar", Owner: mage.BeatUser}

		case mage.Docker:
			args.Spec.ExtraVars["expose_ports"] = config.DefaultPort
			args.Spec.ExtraVars["repository"] = "docker.elastic.co/apm"
			args.Spec.Files["java-attacher.jar"] = mage.PackageFile{Mode: 0750, Source: "build/java-attacher.jar", Owner: mage.BeatUser}

		case mage.Deb, mage.RPM:
			// Update config file owner.
			pf := args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"]
			pf.Owner = mage.BeatUser
			args.Spec.Files["/etc/{{.BeatName}}/{{.BeatName}}.yml"] = pf
			args.Spec.Files["/var/log/{{.BeatName}}"] = mage.PackageFile{Mode: 0750, Source: emptyDir, Owner: mage.BeatUser}
			args.Spec.Files["/usr/share/{{.BeatName}}/bin/java-attacher.jar"] = mage.PackageFile{Mode: 0750, Source: "build/java-attacher.jar", Owner: mage.BeatUser}

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
	ironbank := getIronbankContextName()
	buildDir := filepath.Join("build", "ironbank", ironbank)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return fmt.Errorf("cannot find the folder with the ironbank context")
	}

	distributionsDir := "build/distributions"
	if _, err := os.Stat(distributionsDir); os.IsNotExist(err) {
		err := os.MkdirAll(distributionsDir, 0750)
		if err != nil {
			return fmt.Errorf("cannot create folder for docker artifacts: %+v", err)
		}
	}

	// Save the build context as tar.gz artifact
	tarGzSource, err := TarGz(buildDir, distributionsDir)
	if err != nil {
		return fmt.Errorf("cannot compress the tar.gz file")
	}

	return errors.Wrap(mage.CreateSHA512File(tarGzSource), "failed to create .sha512 file")
}

func Tar(source, target string) error {
	filename := filepath.Base(source)
	target = filepath.Join(target, fmt.Sprintf("%s.tar", filename))
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})
}

func Gzip(source, target string) error {
	reader, err := os.Open(source)
	if err != nil {
		return err
	}

	filename := filepath.Base(source)
	target = filepath.Join(target, fmt.Sprintf("%s.gz", filename))
	writer, err := os.Create(target)
	if err != nil {
		return err
	}
	defer writer.Close()

	archiver := gzip.NewWriter(writer)
	archiver.Name = filename
	defer archiver.Close()

	_, err = io.Copy(archiver, reader)
	return err
}

func TarGz(source, target string) (string, error) {
	filename := filepath.Base(source)
	Tar(source, target)
	tarSource := filepath.Join(target, filename+".tar")

	Gzip(tarSource, target)
	tarGzSource := tarSource + ".gz"

	if err := os.RemoveAll(tarSource); err != nil {
		return "", errors.Wrapf(err, "failed to clean existing build directory %s", tarSource)
	}
	return tarGzSource, nil
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
	ironbank := getIronbankContextName()
	templatesDir := filepath.Join("packaging", "ironbank")

	data := map[string]interface{}{
		"MajorMinor": majorMinor(),
	}

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			target := strings.TrimSuffix(
				filepath.Join("build", ironbank, filepath.Base(path)),
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

func Check() error {
	fmt.Println(">> check: Checking source code for common problems")

	mg.Deps(mage.GoVet, mage.CheckPythonTestNotExecutable, mage.CheckYAMLNotExecutable)

	changes, err := mage.GitDiffIndex()
	if err != nil {
		return errors.Wrap(err, "failed to diff the git index")
	}
	if len(changes) > 0 {
		if mg.Verbose() {
			mage.GitDiff()
		}
		return errors.Errorf("some files are not up-to-date. "+
			"Run 'make fmt update' then review and commit the changes. "+
			"Modified: %v", changes)
	}
	return nil
}

// PythonEnv ensures the Python venv is up-to-date with the beats requrements.txt.
func PythonEnv() error {
	_, err := mage.PythonVirtualenv(false)
	return err
}
