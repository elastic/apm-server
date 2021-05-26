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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/elastic/apm-server/cmd"
	"github.com/elastic/beats/v7/libbeat/common"
)

var versionMapping = map[string]string{
	"7.11": "0.1.0",
	"7.12": "0.1.0",
	"7.13": "0.2.0",
	"7.14": "0.3.0",
	"8.0":  "0.3.0",
}

// Some data streams may not have a counterpart template
// in standalone apm-server, and so it does not make sense
// to maintain a separate fields.yml.
var handwritten = map[string]bool{
	"sampled_traces": true,
}

func main() {
	stackVersion := common.MustNewVersion(cmd.DefaultSettings().Version)
	shortVersion := fmt.Sprintf("%d.%d", stackVersion.Major, stackVersion.Minor)
	packageVersion, ok := versionMapping[shortVersion]
	if !ok {
		panic(errors.New("package can't be generated for current apm-server version"))
	}
	clear(packageVersion)
	inputFields := generateFields(packageVersion)
	for dataStream := range inputFields {
		if err := generatePipelines(packageVersion, dataStream); err != nil {
			log.Fatal(err)
		}
	}
	// TODO(axw) rely on `elastic-package build` to build docs from a template, like in integrations.
	generateDocs(inputFields, packageVersion)
	log.Printf("Package fields and docs generated for version %s (stack %s)", packageVersion, stackVersion.String())
}

func clear(version string) {
	fileInfo, err := ioutil.ReadDir(dataStreamPath(version))
	if err != nil {
		log.Printf("NOTE: if you are adding a new package version, you must create the folder"+
			" `apmpackage/apm/%s/` and copy all the contents from the previous version.", version)
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
		removeFile(ecsFilePath(version, name))
		removeFile(fieldsFilePath(version, name))
		removeDir(pipelinesPath(version, name))
	}
	ioutil.WriteFile(docsFilePath(version), nil, 0644)
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
