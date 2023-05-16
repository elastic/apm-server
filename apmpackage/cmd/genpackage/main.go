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

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/elastic/elastic-agent-libs/version"
)

var (
	outputDir    = flag.String("o", "", "directory into which the package will be rendered (required)")
	pkgVersion   = flag.String("version", "", "integration package version (required)")
	ecsReference = flag.String("ecsref", "", "ECS reference (optional -- defaults to git@v<ecs>)")
	ecsVersion   = flag.String("ecs", "", "ECS version (required)")
)

func generatePackage(pkgfs fs.FS, version, ecsVersion *version.V, ecsReference string) error {
	// Walk files, performing some APM-specific validations and transformations as we go.
	//
	// We assume the target destination does not yet exist.
	return fs.WalkDir(pkgfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, p := range maybeIntervalPath(path) {
			outputPath := filepath.Join(*outputDir, p.Path)
			if d.IsDir() {
				if err := os.Mkdir(outputPath, 0755); err != nil && !errors.Is(err, os.ErrExist) {
					return err
				}
				continue
			} else if strings.HasPrefix(d.Name(), ".") || !d.Type().IsRegular() {
				// Ignore hidden or non-regular files.
				continue
			}
			if p.Interval != "" && strings.HasPrefix(d.Name(), "default_policy") && strings.HasSuffix(d.Name(), ".json") {
				if !strings.Contains(path, p.Interval) {
					// Skip policies that don't match the interval.
					continue
				}
				// Use `default_policy.json` instead of e.g. `default_policy.1m.json` to
				// work around a bug in fleet with more than 1 `.` in policy file name.
				outputPath = strings.Replace(filepath.Join(*outputDir, p.Path), d.Name(), "default_policy.json", -1)
			}
			err := renderFile(pkgfs, path, outputPath, version, ecsVersion, ecsReference, p.Interval)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func renderFile(pkgfs fs.FS, path, outputPath string, version, ecsVersion *version.V, ecsReference, interval string) error {
	content, err := fs.ReadFile(pkgfs, path)
	if err != nil {
		return err
	}
	// Ignore files that have a `generated` prefix.
	if bytes.HasPrefix(content, []byte("generated")) {
		return nil
	}
	if bytes.Contains(content, []byte(`{{ .Interval }}`)) {
		if interval == "" {
			return fmt.Errorf("%s: file contains interval template, but interval is empty", outputPath)
		}
		// Hide data streams with a non-default rollup interval.
		hidden := interval != "1m"
		tpl, err := template.New(path).Parse(string(content))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, pathInterval{Interval: interval, Hidden: hidden}); err != nil {
			return err
		}
		content = buf.Bytes()
	}
	content, err = transformFile(path, content, version, ecsVersion, ecsReference, interval)
	if err != nil {
		return fmt.Errorf("error transforming %q: %w", path, err)
	}
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return err
	}

	type identicalDataStreams struct {
		source, destination string
	}
	// The "traces" and "rum_traces" data streams should have identical fields.
	copyDataStreams := []identicalDataStreams{{
		source:      "data_stream/traces/fields",
		destination: filepath.Join("..", "rum_traces", "fields"),
	}}
	for _, ds := range copyDataStreams {
		if filepath.ToSlash(filepath.Dir(path)) == ds.source {
			originDir := filepath.Dir(filepath.Dir(outputPath))
			destinationDir := filepath.Join(originDir, ds.destination)
			if _, err := os.Stat(destinationDir); errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(destinationDir, 0755); err != nil {
					return err
				}
			}
			copyOutputPath := filepath.Join(destinationDir, filepath.Base(outputPath))
			if err := os.WriteFile(copyOutputPath, content, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

type pathInterval struct {
	Path     string
	Interval string
	Hidden   bool
}

func maybeIntervalPath(path string) []pathInterval {
	if !strings.Contains(path, "_interval_") {
		return []pathInterval{{Path: path}}
	}
	metricInterval := []string{"1m", "10m", "60m"}
	paths := make([]pathInterval, 0, len(metricInterval))
	for _, interval := range metricInterval {
		paths = append(paths, pathInterval{
			Path:     strings.Replace(path, "interval", interval, 1),
			Interval: interval,
		})
	}
	return paths
}

func main() {
	flag.Parse()
	if *outputDir == "" || *pkgVersion == "" || *ecsVersion == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *ecsReference == "" {
		*ecsReference = fmt.Sprintf("git@v%s", *ecsVersion)
	}
	pkgVersion := version.MustNew(*pkgVersion)
	ecsVersion := version.MustNew(*ecsVersion)

	// Locate the apmpackage/apm directory.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("failed to locate source directory")
	}
	pkgdir := filepath.Join(filepath.Dir(file), "..", "..", "apm")

	// Generate a completely rendered _source_ package, which can then be fed to
	// `elastic-agent build` to build the final package for inclusion in package-storage.
	log.Printf("generating integration package v%s in %q", pkgVersion.String(), *outputDir)
	if err := generatePackage(os.DirFS(pkgdir), pkgVersion, ecsVersion, *ecsReference); err != nil {
		log.Fatal(err)
	}
}
