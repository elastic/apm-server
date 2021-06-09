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
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// Some data streams may not have a counterpart template
// in standalone apm-server, and so it does not make sense
// to maintain a separate fields.yml.
var handwritten = map[string]bool{
	"sampled_traces": true,
}

func main() {
	manifestData, err := ioutil.ReadFile(manifestFilePath())
	if err != nil {
		log.Fatal(err)
	}
	var manifest struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		log.Fatal(err)
	}

	clear()
	inputFields := generateFields()
	for dataStream := range inputFields {
		if err := generatePipelines(manifest.Version, dataStream); err != nil {
			log.Fatal(err)
		}
	}
	// TODO(axw) rely on `elastic-package build` to build docs from a template, like in integrations.
	generateDocs(inputFields)
	log.Printf("Package fields and docs generated for version %s", manifest.Version)
}

func clear() {
	fileInfo, err := ioutil.ReadDir(dataStreamPath())
	if err != nil {
		panic(err)
	}
	for _, f := range fileInfo {
		if !f.IsDir() {
			continue
		}
		name := f.Name()
		if handwritten[name] {
			continue
		}
		removeFile(ecsFilePath(name))
		removeFile(fieldsFilePath(name))
		removeDir(pipelinesPath(name))
	}
	ioutil.WriteFile(docsFilePath(), nil, 0644)
}

func removeFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
}

func removeDir(path string) {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
}
