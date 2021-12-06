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
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	outputDir = flag.String("o", "", "directory into which the package will be rendered (required)")
	version   = flag.String("version", "", "integration package version (required)")
)

func generatePackage(pkgfs fs.FS, version *common.Version) error {
	// Walk files, performing some APM-specific validations and transformations as we go.
	//
	// We assume the target destination does not yet exist.
	if err := fs.WalkDir(pkgfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		outputPath := filepath.Join(*outputDir, path)
		if d.IsDir() {
			if err := os.Mkdir(outputPath, 0755); err != nil {
				return err
			}
			return nil
		} else if strings.HasPrefix(d.Name(), ".") || !d.Type().IsRegular() {
			// Ignore hidden or non-regular files.
			return nil
		}
		return renderFile(pkgfs, path, outputPath, version)
	}); err != nil {
		return err
	}
	return nil
}

func renderFile(pkgfs fs.FS, path, outputPath string, version *common.Version) error {
	content, err := fs.ReadFile(pkgfs, path)
	if err != nil {
		return err
	}
	content, err = transformFile(path, content, version)
	if err != nil {
		return fmt.Errorf("error transforming %q: %w", path, err)
	}
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return err
	}
	// The "traces" and "rum_traces" data streams should have identical fields.
	//
	// Copy all files in `data_stream/traces/fields` to `data_stream/rum_traces/fields`.
	if filepath.ToSlash(filepath.Dir(path)) == "data_stream/traces/fields" {
		tracesDir := filepath.Dir(filepath.Dir(outputPath))
		rumTracesFieldsDir := filepath.Join(tracesDir, "..", "rum_traces", "fields")
		copyOutputPath := filepath.Join(rumTracesFieldsDir, filepath.Base(outputPath))
		if err := os.WriteFile(copyOutputPath, content, 0644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()
	if *outputDir == "" || *version == "" {
		flag.Usage()
		os.Exit(2)
	}
	version := common.MustNewVersion(*version)

	// Locate the apmpackage/apm directory.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("failed to locate source directory")
	}
	pkgdir := filepath.Join(filepath.Dir(file), "..", "..", "apm")

	// Generate a completely rendered _source_ package, which can then be fed to
	// `elastic-agent build` to build the final package for inclusion in package-storage.
	log.Printf("generating integration package v%s in %q", version, *outputDir)
	if err := generatePackage(os.DirFS(pkgdir), version); err != nil {
		log.Fatal(err)
	}
}
